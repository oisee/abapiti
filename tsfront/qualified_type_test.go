package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Qualified assertions used to panic before reaching the checker because the
// as-const probe called Node.Text on a QualifiedName instead of an identifier.
func TestQualifiedTypeAssertion(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{
		"defs.ts": `export class Box { value: number = 7; }
export type Number = number;`,
		"probe.ts": `import * as N from "./defs";
export class Probe {
static number(): number { return (42 as N.Number); }
static identity(box: N.Box): N.Box { return box as N.Box; }
static literal(): number { return 3 as const; }
}`,
	}, []string{"defs.ts", "probe.ts"})
	names := hir.NewNames()
	main := "box := " + names.Get("new.defs.ts.Box") + "();fmt.Println(" + goEntry("probe.ts.Probe", "number") + "()," + goEntry("probe.ts.Probe", "identity") + "(box)==box," + goEntry("probe.ts.Probe", "literal") + "())"
	if got := runGoHIR(t, p, main); got != "42 true 3\n" {
		t.Fatal(got)
	}
}

func TestQualifiedTypeAssertionRefusesUnprovenCast(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"defs.ts": `export class Base {}
export class Child extends Base { value: number = 7; }
export {Child as const};`,
		"probe.ts": `import * as N from "./defs";
export function cast(x: N.Base): N.Child { return x as N.const; }`,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`,
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, trap := range []bool{false, true} {
		prog, diags, err := p.LowerWithOptions([]string{"defs.ts", "probe.ts"}, LowerOptions{TrapOnFailure: trap})
		if err != nil {
			t.Fatal(err)
		}
		if !trap {
			found := false
			for _, d := range diags {
				if d.Category == "unsupported-assertion" && strings.Contains(d.Loc, "probe.ts:") {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing cast refusal: %v", diags)
			}
		} else {
			if hasBlocking(diags) {
				t.Fatal(diags)
			}
			if errors := hir.Verify(prog); len(errors) > 0 {
				t.Fatal(errors)
			}
			if !strings.Contains(hir.Dump(prog), "type assertion requires checker-proven narrowing") {
				t.Fatal("missing trap")
			}
		}
	}
}
