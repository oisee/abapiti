package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The same observations run on Node, Go, and the collected ABAP Unit fixture.
func TestTypeofCallableMarkerNodeOracle(t *testing.T) {
	source := `class Value { method(): string {return "ok";} }
export class Probe {
 static tag(x:any): string {return typeof x;}
 static named(): string {return "ok";}
 static run(): string {
  const captured="ok";
  const arrow=():string=>captured;
  const fn=function():string {return "ok";};
  const thunk=():void=>{};
  const instance=new Value();
  const method=instance.method;
  const staticMethod=Probe.named;
  const constructor:any=Value;
  const plain:any={value:"ok"};
  const array:any=[];
  return Probe.tag(thunk)+"|"+Probe.tag(arrow)+"|"+Probe.tag(fn)+"|"+Probe.tag(method)+"|"+Probe.tag(staticMethod)+"|"+Probe.tag(constructor)+"|"+Probe.tag(instance)+"|"+Probe.tag(plain)+"|"+Probe.tag(array)+"|"+Probe.tag(null)+"|"+Probe.tag(undefined)+"|"+Probe.tag("ok")+"|"+Probe.tag(7)+"|"+Probe.tag(false);
 }
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := strings.TrimSpace(string(out))
	if got := runGoHIR(t, p, "fmt.Print("+goEntry("probe.ts.Probe", "run")+"().String())"); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("probe.ts.Probe")
	marker := names.Get("runtime.callable")
	if len(marker) > 30 || files[marker+".intf.abap"] == "" {
		t.Fatal("missing callable marker")
	}
	src := files[owner+".clas.abap"]
	if !strings.Contains(src, "->oval IS INSTANCE OF "+marker+".") {
		t.Fatal("missing constant-size callable test")
	}
	for _, c := range p.Classes {
		if strings.HasPrefix(c.Name, "closure.") && strings.Contains(files[names.Get(c.Name)+".clas.abap"], "INTERFACES "+marker+".") {
			if strings.Contains(src, "->oval IS INSTANCE OF "+names.Get(c.Name)+".") {
				t.Fatal("typeof still enumerates closure classes")
			}
		}
	}
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_typeof DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS observations FOR TESTING.
ENDCLASS.
CLASS ltcl_typeof IMPLEMENTATION.
METHOD observations.
cl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = %s ).
ENDMETHOD.
ENDCLASS.
`, owner, names.Get("member.run"), "`"+want+"`")
}

// Native function values and first-class bind expressions have no proven HIR
// callable representation yet. They must keep a diagnostic, never become an
// ordinary object whose typeof silently returns the wrong answer.
func TestTypeofUnprovenCallableRefusals(t *testing.T) {
	for _, value := range []string{"Math.abs", "new Value().method.bind(new Value())"} {
		t.Run(value, func(t *testing.T) {
			source := `class Value { method(): string {return "ok";} }
export class Probe { static run(): string {const f:any=` + value + `;return typeof f;} }`
			dir := sourceProbe(t, source)
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, ds, err := p.Lower([]string{"input.ts"})
			if err != nil || !hasBlocking(ds) {
				t.Fatalf("unproven callable needs explicit refusal: %v %v", err, ds)
			}
			path := filepath.Join(dir, "oracle.ts")
			if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("node", path).CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != "function" {
				t.Fatalf("Node: %v %s", err, out)
			}
		})
	}
}
