package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoInstanceOfUnnarrowedOperandNodeOracle(t *testing.T) {
	// Empty sibling classes are structurally identical to TS but nominally
	// distinct at runtime. Flow types in compound tests must not cast before
	// the ancestry test (including repeated tests and false branches).
	source := `class Base {} class A extends Base {} class B extends Base {} class C extends Base {}
 class Probe {
 static calls=0;
 static once(b:Base):Base {Probe.calls++;return b;}
 static check(b:Base):string {
 const a=b instanceof A;
 const either=(b instanceof A && true)||(b instanceof B && true);
 const repeated=b instanceof A || b instanceof A;
 const later=true && b instanceof C;
 return a+":"+either+":"+repeated+":"+later+":"+(Probe.once(b) instanceof B);
 }
 static run():string {return Probe.check(new A())+"|"+Probe.check(new B())+"|"+Probe.check(new C())+"|"+Probe.check(new Base())+"|"+Probe.calls;}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
