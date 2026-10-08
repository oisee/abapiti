package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront/overrides"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLowerSourcePinnedOverride(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true},"files":["probe.ts"]}`), 0600)
	file := filepath.Join(dir, "probe.ts")
	entry := overrides.Entry{ID: "probe-literal", Key: overrides.Key{File: "probe.ts", Symbol: "Probe.run", Kind: "KindNumericLiteral"}, SHA256: overrides.Fingerprint("42"), Rationale: "test registry integration", Expression: func() *hir.Expr { return hir.L(hir.T(hir.Number), 7) }}
	registry, err := overrides.New(entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{`export class Probe { run(): number { return 42; } }`, `export class Probe { run(): number { return 43; } }`, `export class Probe { run(): number { return 0; } }`, `export class Probe { run(): number { return 42 + 42; } }`} {
		os.WriteFile(file, []byte(source), 0600)
		p, err := Load(config)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := p.LowerWithOverrides([]string{"probe.ts"}, registry)
		if strings.Contains(source, "return 42;") {
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(hir.Dump(prog), "7") {
				t.Fatal("override not applied")
			}
			if errs := hir.Verify(prog); len(errs) > 0 {
				t.Fatal(errs)
			}
		} else if err == nil || !strings.Contains(err.Error(), "override probe-literal is stale") {
			t.Fatal("changed target was accepted", err)
		}
	}
}

func TestPinnedAnnotationPatternsAndMethodSignatures(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	method := `run(input: Iterable<number>): number { let n = 0; for (const v of input) { n += v; } return n + Date.now(); }`
	source := `export interface Source { get(): Generator<number, void, undefined>; } export class Probe { ` + method + ` consume(source: Source): number { return this.run(source.get()); } }`
	registry, err := overrides.New(
		overrides.Entry{ID: "annotation-probe", Key: overrides.Key{File: "probe.ts", Symbol: "Probe.run", Kind: "KindMethodDeclaration"}, SHA256: overrides.Fingerprint(method), Rationale: "reviewed snapshot consumer annotation and clock", Patterns: &overrides.Patterns{Annotations: map[string]hir.Type{"Iterable<number>": hir.T(hir.Array, hir.T(hir.Number))}, Expressions: map[string]func() *hir.Expr{"Date.now()": overrides.TelemetryClock}}},
		overrides.Entry{ID: "signature-probe", Key: overrides.Key{File: "probe.ts", Symbol: "Source.get", Kind: "KindMethodSignature"}, SHA256: overrides.Fingerprint(`get(): Generator<number, void, undefined>;`), Rationale: "reviewed producer signature", Result: func() hir.Type { return hir.T(hir.Array, hir.T(hir.Number)) }},
	)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "probe.ts")
	for _, test := range []struct {
		source           string
		stale, unadapted bool
	}{
		{source: source},
		{source: source + ` export interface Other { get(): Generator<number, void, undefined>; }`, unadapted: true},
		{source: strings.Replace(source, "Iterable<number>", "Iterable<string>", 1), stale: true},
	} {
		if err := os.WriteFile(file, []byte(test.source), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := Load(config)
		if err != nil {
			t.Fatal(err)
		}
		prog, diags, err := p.LowerWithOverrides([]string{"probe.ts"}, registry)
		if test.stale {
			if err == nil || !strings.Contains(err.Error(), "annotation-probe is stale") {
				t.Fatal("changed annotation did not fail", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if test.unadapted {
			if !hasBlocking(diags) {
				t.Fatal("unadapted Generator was globally erased")
			}
		} else {
			if hasBlocking(diags) {
				t.Fatal(diags)
			}
			if errs := hir.Verify(prog); len(errs) > 0 {
				t.Fatal(errs, hir.Dump(prog))
			}
			if !strings.Contains(hir.Dump(prog), "clock.telemetry") {
				t.Fatal("combined expression override missing")
			}
		}
	}
}
