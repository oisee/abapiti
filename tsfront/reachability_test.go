package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestCoverageTrapAndStaleSpan(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	body := "dead(): number { return new Date().getTime(); }"
	source := "export class Probe { live(): number { return 7; } " + body + " }"
	file := filepath.Join(dir, "probe.ts")
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	start := strings.Index(source, body)
	coverage := &Reachability{Schema: 1, Workloads: []string{"unit"}, Spans: []CoverageSpan{{File: "probe.ts", Start: start, End: start + len(body), Kind: "MethodDeclaration", Line: 1, SHA256: overrides.Fingerprint(body)}}}
	registry, err := overrides.New()
	if err != nil {
		t.Fatal(err)
	}
	load := func() *Program {
		p, err := Load(config)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := load()
	prog, diags, err := p.LowerWithReachability([]string{"probe.ts"}, registry, coverage)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	if !strings.Contains(hir.Dump(prog), "trap probe.ts:1") {
		t.Fatal(hir.Dump(prog))
	}
	files, err := abap.Emit(prog)
	if err != nil {
		t.Fatal(err)
	}
	var code string
	for _, f := range files {
		code += f
	}
	if !strings.Contains(code, "DATA source_location TYPE string") || !strings.Contains(code, "RAISE EXCEPTION") || !strings.Contains(code, "probe.ts:1") {
		t.Fatal("missing dedicated located exception", code)
	}
	coverage.Spans[0].Executed = true
	_, diags, err = p.LowerWithReachability([]string{"probe.ts"}, registry, coverage)
	if err != nil {
		t.Fatal(err)
	}
	blocked := false
	for _, d := range diags {
		blocked = blocked || !strings.HasPrefix(d.Category, "note-")
	}
	if !blocked {
		t.Fatal("executed Date body was suppressed")
	}
	coverage.Spans[0].Executed = false
	for _, changed := range []string{strings.Replace(source, "getTime()", "getDate()", 1), strings.Replace(source, body, "", 1)} {
		if err := os.WriteFile(file, []byte(changed), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := load().LowerWithReachability([]string{"probe.ts"}, registry, coverage); err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatal("changed/deleted span accepted", err)
		}
	}
}

func TestTemplateInterpolation(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "tsconfig.json")
	source := "export class Probe { run(s: string, n: number, b: boolean): string { return ` first ${s}\\n${n}/${b} last `; } }"
	if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(config)
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithOverrides([]string{"probe.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	dump := hir.Dump(prog)
	for _, want := range []string{"string.concat", "number.toString", "true", "false", " last "} {
		if !strings.Contains(dump, want) {
			t.Fatal("template missing", want, dump)
		}
	}
}

// An upstream-only unsupported body contributes neither an execution root
// nor body dependencies, but its ABI remains when the class is reached.
func TestWorkloadProvenanceAndDeclarationGraph(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "tsconfig.json")
	source := `export interface Shape { value: string; }
export class Probe {
 live(): Shape { return {value: "ok"}; }
 dead(): number { return new Date().getTime(); }
}
export class Removed { dead(): number { return new Date().getTime(); } }
export class OnlyType { value: string = "hidden"; }
export class LiveValue { value: string = "constructed"; }
export class Consumer { build(unused: OnlyType): LiveValue { return new LiveValue(); } }`
	if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	coverage := &Reachability{Schema: 2, Workloads: []string{"deployment", "upstream"}}
	for _, body := range []string{`live(): Shape { return {value: "ok"}; }`, `dead(): number { return new Date().getTime(); }`} {
		offset := 0
		for {
			index := strings.Index(source[offset:], body)
			if index < 0 {
				break
			}
			start := offset + index
			workload := "UPSTREAM"
			if strings.HasPrefix(body, "live") {
				workload = "DEPLOYMENT"
			}
			coverage.Spans = append(coverage.Spans, CoverageSpan{File: "probe.ts", Start: start, End: start + len(body), Kind: "MethodDeclaration", Line: strings.Count(source[:start], "\n") + 1, SHA256: overrides.Fingerprint(body), Executed: true, Workloads: []string{workload}})
			offset = start + len(body)
		}
	}
	p, err := Load(config)
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"probe.ts"}, nil, coverage)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	dump := hir.Dump(prog)
	if strings.Contains(dump, "Removed") || !strings.Contains(dump, "Shape") || !strings.Contains(dump, "trap probe.ts:4") {
		t.Fatal(dump)
	}
	var onlyType, liveValue *hir.Class
	for _, c := range prog.Classes {
		if strings.HasSuffix(c.Name, ".OnlyType") {
			onlyType = c
		}
		if strings.HasSuffix(c.Name, ".LiveValue") {
			liveValue = c
		}
	}
	if onlyType == nil || onlyType.Ctor == nil || onlyType.Ctor.Body == nil || onlyType.Ctor.Body.List[0].Kind != hir.Trap {
		t.Fatal("type-only constructor must trap", dump)
	}
	if liveValue == nil || liveValue.Ctor == nil || liveValue.Ctor.Body == nil || strings.Contains(hir.Dump(&hir.Program{Classes: []*hir.Class{liveValue}}), "trap") {
		t.Fatal("runtime initializer was trapped", dump)
	}
	coverage.Spans[1].Workloads = []string{"NEGATIVE"}
	_, diags, err = p.LowerWithReachability([]string{"probe.ts"}, nil, coverage)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlocking(diags) {
		t.Fatal("negative executed body was trapped")
	}
	coverage.Spans[1].Workloads = []string{"INVALID"}
	if _, _, err = p.LowerWithReachability([]string{"probe.ts"}, nil, coverage); err == nil {
		t.Fatal("unknown workload class accepted")
	}
	coverage.Spans[1].Workloads = []string{"UPSTREAM"}
	coverage.Spans[1].Executed = false
	if _, _, err = p.LowerWithReachability([]string{"probe.ts"}, nil, coverage); err == nil {
		t.Fatal("inconsistent union execution accepted")
	}
}
