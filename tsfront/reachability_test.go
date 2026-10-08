package tsfront

import (
	"fmt"
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
	coverage.Spans[1].Workloads = []string{"OBSERVATION"}
	_, diags, err = p.LowerWithReachability([]string{"probe.ts"}, nil, coverage)
	if err != nil || !hasBlocking(diags) {
		t.Fatal("observation body did not remain live", err)
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

func TestCoverageWorkloadInputsValidatedBeforePruning(t *testing.T) {
	dir := t.TempDir()
	input, deps := filepath.Join(dir, "input"), filepath.Join(dir, "deps")
	for _, path := range []string{input, deps} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	paths := map[string]string{"input/probe.abap": filepath.Join(input, "probe.abap"), "dependencies/dep.abap": filepath.Join(deps, "dep.abap"), "config.json": filepath.Join(dir, "config.json"), "negative-issues.json": filepath.Join(dir, "negative.json")}
	coverage := &Reachability{Schema: 2, Workloads: []string{"north-star"}, CurrentInputs: &CoverageInputs{InputDir: input, DependenciesDir: deps, ConfigPath: paths["config.json"], NegativesPath: paths["negative-issues.json"]}}
	for name, path := range paths {
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		coverage.Inputs = append(coverage.Inputs, struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		}{name, overrides.Fingerprint(name)})
	}
	config := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(config, []byte(`{"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(`export class Probe { run(): number { return 1; } }`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(config)
	if err != nil {
		t.Fatal(err)
	}
	lower := func() error { _, _, err := p.LowerWithReachability([]string{"probe.ts"}, nil, coverage); return err }
	if err := lower(); err != nil {
		t.Fatal(err)
	}
	for name, path := range paths {
		if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := lower(); err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("changed input %s was accepted: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	extra := filepath.Join(input, "extra.abap")
	if err := os.WriteFile(extra, []byte("added"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := lower(); err == nil {
		t.Fatal("added input accepted")
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	coverage.CurrentInputs = nil
	if err := lower(); err == nil {
		t.Fatal("unbound inputs accepted")
	}
}

// Named namespace references retain exactly the resolved export; using a
// namespace as a value must conservatively retain its whole export inventory.
func TestDeclarationGraphNamespaceEdges(t *testing.T) {
	for _, reflective := range []bool{false, true} {
		t.Run(fmt.Sprint(reflective), func(t *testing.T) {
			dir := t.TempDir()
			members := `export class Wanted { static value(): number { return 1; } }
export class Unused { static dead(): number { return new Date().getTime(); } }`
			use := `import * as NS from "./members";
export function run(): number { return NS.Wanted.value(); }`
			if reflective {
				use = `import * as NS from "./members";
export function run(): number { const all = NS; return all.Wanted.value(); }`
			}
			for name, text := range map[string]string{"members.ts": members, "use.ts": use, "tsconfig.json": `{"compilerOptions":{"strict":true},"files":["members.ts","use.ts"]}`} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			body := `static dead(): number { return new Date().getTime(); }`
			start := strings.Index(members, body)
			coverage := &Reachability{Schema: 2, Workloads: []string{"deployment"}, Spans: []CoverageSpan{{File: "members.ts", Start: start, End: start + len(body), Kind: "MethodDeclaration", Line: 2, SHA256: overrides.Fingerprint(body), Executed: true, Workloads: []string{"UPSTREAM"}}}}
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			prog, _, err := p.LowerWithReachability([]string{"members.ts", "use.ts"}, nil, coverage)
			if err != nil {
				t.Fatal(err)
			}
			dump := hir.Dump(prog)
			if !strings.Contains(dump, "Wanted") || strings.Contains(dump, "Unused") != reflective {
				t.Fatal(dump)
			}
		})
	}
}
