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
// Template text is opaque, but embedded expressions can contain real calls
// and nested templates. Keep quoted literals opaque inside expressions too.
func superGuardCode(source string) string {
	var code strings.Builder
	i := 0
	var scan func(bool)
	scan = func(expression bool) {
		for i < len(source) {
			ch := source[i]
			i++
			switch ch {
			case '}':
				if expression {
					code.WriteByte(' ')
					return
				}
				code.WriteByte(ch)
			case '\'', '`':
				code.WriteByte(ch)
				for i < len(source) {
					c := source[i]
					i++
					code.WriteByte(c)
					if c == ch {
						if i < len(source) && source[i] == ch {
							code.WriteByte(source[i])
							i++
						} else {
							break
						}
					}
				}
			case '|':
				code.WriteByte(' ')
				for i < len(source) {
					c := source[i]
					i++
					if c == '\\' && i < len(source) {
						i++
					} else if c == '{' {
						scan(true)
					} else if c == '|' {
						break
					}
				}
				code.WriteByte(' ')
			default:
				code.WriteByte(ch)
			}
		}
	}
	scan(false)
	return code.String()
}

func superMethodViolations(files map[string]string) []string {
	tokens := regexp.MustCompile("(?i)'(?:''|[^'])*'|`(?:``|[^`])*`|[a-z_][a-z_0-9~]*|->|[.]")
	var violations []string
	for file, source := range files {
		words := tokens.FindAllString(strings.ToLower(superGuardCode(source)), -1)
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
	for _, source := range []string{
		`METHOD m. text = |. METHOD x.|. super->x( ). ENDMETHOD.`,
		`METHOD m. text = |\| \{ \} . METHOD x.|. super->x( ). ENDMETHOD.`,
		`METHOD m. text = |text { super->x( ) }|. ENDMETHOD.`,
		`METHOD m. text = |text { |nested { super->x( ) }| }|. ENDMETHOD.`,
		`METHOD m. text = |text { '}' } . METHOD x.|. super->x( ). ENDMETHOD.`,
	} {
		if bad := superMethodViolations(map[string]string{"bad": source}); len(bad) != 1 || bad[0] != "bad: METHOD m calls super->x" {
			t.Fatalf("%s: %v", source, bad)
		}
	}
	assertSuperSameMethod(t, map[string]string{"ok": `METHOD m. text = |super->x( ) \| \{ \} { super->m( ) } { |nested super->x( )| }|. ENDMETHOD.`})
}

func TestOptionalPrimitiveSuperOverrideRejected(t *testing.T) {
	for _, primitive := range []string{"number", "string", "boolean"} {
		t.Run(primitive, func(t *testing.T) {
			prog := lowerStatementsProbe(t, map[string]string{"probe.ts": fmt.Sprintf(`
 export class Base { f(x?: %s): Base { return this; } }
 export class Child extends Base { f(x: %s): Child { super.f(x); return this; } }
 `, primitive, primitive)}, []string{"probe.ts"})
			files, _, err := abap.EmitNamed(prog)
			if err == nil || !strings.Contains(err.Error(), "narrowing an inherited optional primitive cannot preserve undefined") || len(files) != 0 {
				t.Fatalf("expected blocking optional primitive diagnostic with no output, got %v (%d files)", err, len(files))
			}
		})
	}
}
func TestCovariantSuperOverride(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Base { set(x: Base): Base { return x; } }
 export class Child extends Base { set(x: Base): Child { super.set(x); return this; } }
 export class Grandchild extends Child { set(x: Base): Grandchild { super.set(x); return this; } }
 export class VoidBase { set(x: Base): void {} }
 export class VoidChild extends VoidBase { set(x: Base): VoidChild { super.set(x); return this; } }
 export class OptionalBase {
  value: number = 99;
  set(x?: number): OptionalBase { this.value = x === undefined ? -1 : x; return this; }
 }
 export class OptionalChild extends OptionalBase {
  set(x?: number): OptionalChild { super.set(x); return this; }
 }
 export class Probe { static run(): boolean {
  const g = new Grandchild(); const b: Base = g; const x = new Base();
  const v = new VoidChild();
  const o = new OptionalChild(); const optionalBase: OptionalBase = o;
  const omitted = optionalBase.set() === o && o.value === -1;
  const zero = o.set(0) === o && o.value === 0;
  const supplied = optionalBase.set(42) === o && o.value === 42;
  const explicit = optionalBase.set(undefined) === o && o.value === -1;
  return g instanceof Child && b.set(x) === g && g.set(x) === g && v.set(x) === v && omitted && zero && supplied && explicit;
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
	// Nothing from Child down redefines x: super.x() is me->x( ).
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Base { x(): number { return 1; } }
 export class Child extends Base { m(): number { return super.x(); } }
 `}, []string{"probe.ts"})
	files, err := abap.Emit(prog)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, src := range files {
		if strings.Contains(src, "super->") {
			t.Fatal("SUPER-> to another method was emitted")
		}
		found = found || strings.Contains(src, "me->")
	}
	if !found {
		t.Fatal("expected the inherited call through me")
	}
	// A subclass redefining x would capture me->x( ): still rejected.
	prog = lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Base { x(): number { return 1; } }
 export class Child extends Base { m(): number { return super.x(); } }
 export class Grand extends Child { x(): number { return 3; } }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err == nil || !strings.Contains(err.Error(), "previous implementation of the same method") {
		t.Fatalf("expected blocking super diagnostic, got %v", err)
	}
}
