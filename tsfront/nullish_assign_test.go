package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// a ??= b on a field of any object, as a statement and as a value: the
// receiver is evaluated once and b only while the field is absent.
func TestNullishAssignFieldsNodeOracle(t *testing.T) {
	source := `class Box {items?: Map<string, number>; label?: string;}
 class Probe {
 static reads=0;
 static made=0;
 static box=new Box();
 static target():Box {Probe.reads++;return Probe.box;}
 static fresh():Map<string, number> {Probe.made++;return new Map<string, number>();}
 static run():string {
 const b=new Box();
 b.label ??= "first";
 b.label ??= "second";
 const m=b.items ??= Probe.fresh();
 m.set("a",1);
 const again=b.items ??= Probe.fresh();
 Probe.target().label ??= "x";
 Probe.target().label ??= "y";
 const v=Probe.target().items ??= Probe.fresh();
 return b.label+":"+again.size+":"+Probe.made+":"+Probe.reads+":"+Probe.box.label+":"+v.size;
 }
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
