package tsfront

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestRulesCallbackSourcePins(t *testing.T) {
	specs := []struct{ file, symbol, kind, start, end string }{
		{"src/rules/uncaught_exception.ts", "UncaughtException.setSinkedFromMethod", "KindMethodDeclaration", "private setSinkedFromMethod(", "\n  }"},
		{"src/abap/types/method_definition.ts", "MethodDefinition", "KindClassDeclaration", "export class MethodDefinition ", "\n}"},
		{"src/rules/parser_702_chaining.ts", "Parser702Chaining.ensureSourceHasNoProceduralKeywords", "KindMethodDeclaration", "private ensureSourceHasNoProceduralKeywords(", "\n  }"},
		{"src/rules/short_case.ts", "ShortCase", "KindClassDeclaration", "export class ShortCase extends", "\n}"},
	}
	zr, err := gzip.NewReader(bytes.NewReader(EmbeddedAbaplintArchive()))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	sources := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range specs {
			if h.Name == "packages/core/"+s.file {
				raw, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				sources[s.file] = string(raw)
			}
		}
	}
	registry, err := RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range specs {
		t.Run(s.symbol, func(t *testing.T) {
			text := sources[s.file]
			start := strings.Index(text, s.start)
			if start < 0 {
				t.Fatal("missing source")
			}
			end := strings.Index(text[start:], s.end)
			if end < 0 {
				t.Fatal("missing end")
			}
			span := text[start : start+end+len(s.end)]
			key := overrides.Key{File: s.file, Symbol: s.symbol, Kind: s.kind}
			entry, ok, err := registry.Lookup(key, span, s.file)
			if !ok || err != nil {
				t.Fatal(ok, err)
			}
			if entry.Patterns == nil || len(entry.Patterns.DenseCallbacks) == 0 && !entry.Patterns.SourceGuard {
				t.Fatal("missing proof")
			}
			for call := range entry.Patterns.DenseCallbacks {
				if !strings.Contains(span, call) {
					t.Fatal("missing call", call)
				}
			}
			if _, _, err := registry.Lookup(key, span+" ", s.file); err == nil {
				t.Fatal("stale pin accepted")
			}
		})
	}
}

func TestRulesDenseFindGoMatchesNode(t *testing.T) {
	source := `export class Probe {
 static calls: string = "";
 static accept(value: string): boolean { Probe.calls += "[" + value + "]"; return value === ""; }
 static run(): string {
  const values = ["", "X"];
  const found = values.find(value => Probe.accept(value));
  const missing = values.find(value => value === "missing");
  const empty: string[] = [];
  const none = empty.find(value => value === "");
  const block = values.find(value => { return value === "X"; });
  return (found === undefined) + "/" + (missing === undefined) + "/" + (none === undefined) + "/" + block + "/" + Probe.calls;
 }
}`
	dir := sourceProbe(t, source)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]bool{
		"values.find(value => Probe.accept(value))":       true,
		`values.find(value => value === "missing")`:       true,
		`empty.find(value => value === "")`:               true,
		`values.find(value => { return value === "X"; })`: true,
	}
	registry, err := overrides.New(overrides.Entry{ID: "rules-find-fixture", Key: overrides.Key{File: "input.ts", Symbol: "Probe", Kind: "KindClassDeclaration"}, SHA256: overrides.Fingerprint(source), Rationale: "fresh dense literals; predicates only read receiver and record visits separately", Patterns: &overrides.Patterns{DenseCallbacks: calls}})
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithOverrides([]string{"input.ts"}, registry)
	if err != nil || hasBlocking(diags) {
		t.Fatal(err, diags)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", oracle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
	// A general primitive find remains refused without a receiver proof.
	_, unproven, err := p.LowerWithOverrides([]string{"input.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlocking(unproven) {
		t.Fatal("unproven receiver accepted")
	}
}

func TestRulesDenseForEachGoMatchesNode(t *testing.T) {
	source := `export class Provider {
  private values: string[] = [];
  constructor() { this.values.push("A"); this.values.push("B"); }
  getRaising(): readonly string[] { return this.values; }
 }
export class Probe {
 private sinked: string[] | undefined;
 collect(present: boolean): string {
  const def = new Provider();
  this.sinked = present ? [] : undefined;
  def.getRaising().forEach(r => this.sinked?.push(r));
  return this.sinked ? this.sinked.join(",") : "absent";
 }
 static run(): string { const probe = new Probe(); return probe.collect(true) + "/" + probe.collect(false); }
}`
	dir := sourceProbe(t, source)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(source, "export class Probe")
	registry, err := overrides.New(overrides.Entry{ID: "rules-forEach-fixture", Key: overrides.Key{File: "input.ts", Symbol: "Probe", Kind: "KindClassDeclaration"}, SHA256: overrides.Fingerprint(source[start:]), Rationale: "producer constructs a private dense array, callback writes a fresh independent sink; optional sink short circuits", Patterns: &overrides.Patterns{DenseCallbacks: map[string]bool{"def.getRaising().forEach(r => this.sinked?.push(r))": true}}})
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithOverrides([]string{"input.ts"}, registry)
	if err != nil || hasBlocking(diags) {
		t.Fatal(err, diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", oracle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
}
