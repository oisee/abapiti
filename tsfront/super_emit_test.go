package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

// Scan ABAP tokens rather than lines: wrapping must not hide a SUPER call.
func superMethodViolations(files map[string]string) []string {
	tokens := regexp.MustCompile("(?i)'(?:''|[^'])*'|`(?:``|[^`])*`|[a-z_][a-z_0-9~]*|->|[.]")
	var violations []string
	for file, source := range files {
		words := tokens.FindAllString(strings.ToLower(source), -1)
		method := ""
		for i, word := range words {
			if word == "method" && (i == 0 || words[i-1] == ".") && i+1 < len(words) {
				method = words[i+1]
			}
			if word == "endmethod" {
				method = ""
			}
			if word == "super" && i+2 < len(words) && words[i+1] == "->" && words[i+2] != method {
				violations = append(violations, file+": METHOD "+method+" calls super->"+words[i+2])
			}
		}
	}
	return violations
}
func assertSuperSameMethod(t *testing.T, files map[string]string) {
	t.Helper()
	if bad := superMethodViolations(files); len(bad) != 0 {
		t.Fatal(strings.Join(bad, "\n"))
	}
}
func TestSuperSameMethodGuard(t *testing.T) {
	assertSuperSameMethod(t, map[string]string{"ok": "METHOD constructor. super->constructor( ). ENDMETHOD. METHOD x. CALL METHOD z_class=>initialize. super->\nx( ). text = 'super->y( )'. ENDMETHOD."})
	if bad := superMethodViolations(map[string]string{"bad": "METHOD m. super->\nx( ). ENDMETHOD."}); len(bad) != 1 {
		t.Fatal(bad)
	}
}
func TestCovariantSuperOverride(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Base { set(x: Base): Base { return x; } }
 export class Child extends Base { set(x: Base): Child { super.set(x); return this; } }
 export class Grandchild extends Child { set(x: Base): Grandchild { super.set(x); return this; } }
 export class VoidBase { set(x: Base): void {} }
 export class VoidChild extends VoidBase { set(x: Base): VoidChild { super.set(x); return this; } }
 export class Probe { static run(): boolean {
  const g = new Grandchild(); const b: Base = g; const x = new Base();
  const v = new VoidChild();
  return g instanceof Child && b.set(x) === g && g.set(x) === g && v.set(x) === v;
 } }
 `}, []string{"probe.ts"})
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	assertSuperSameMethod(t, files)
	for _, c := range prog.Classes {
		if c.Super == "" {
			continue
		}
		source := files[names.Get(c.Name)+".clas.abap"]
		slot := names.Get("member.set")
		if !strings.Contains(source, "METHOD "+slot+".") || !strings.Contains(source, "super->"+slot) || !strings.Contains(source, "me->"+slot) || !strings.Contains(source, " ?= ") {
			t.Fatal(source)
		}
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		files[names.Get("probe.ts.Probe")+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_covariant DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS covariant FOR TESTING.
ENDCLASS.
CLASS ltcl_covariant IMPLEMENTATION.
METHOD covariant.
DATA actual TYPE abap_bool.
actual = %s=>%s( ).
cl_abap_unit_assert=>assert_equals( act = actual exp = abap_true ).
ENDMETHOD.
ENDCLASS.
`, names.Get("probe.ts.Probe"), names.Get("member.run"))
		dir := filepath.Join(out, "covariant-super")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCrossMethodSuperRejected(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Base { x(): number { return 1; } }
 export class Child extends Base { m(): number { return super.x(); } }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err == nil || !strings.Contains(err.Error(), "previous implementation of the same method") {
		t.Fatalf("expected blocking super diagnostic, got %v", err)
	}
}
