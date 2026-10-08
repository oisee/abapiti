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
