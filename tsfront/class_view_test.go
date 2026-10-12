package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoCheckedNominalClassViewNodeOracle(t *testing.T) {
	source := `class Base {value=1;} class Child extends Base {read():number {return this.value;}}
 class Other extends Base {}
 class Probe {
 static count=0;
 static once(value:Base):Base {Probe.count++;return value;}
 static view(value:Base):Child {return Probe.once(value) as Child;}
 static run():string {const child=new Child();const base:Base=child;const viewed=Probe.view(base);viewed.value=7;return (viewed===child)+":"+child.read()+":"+Probe.count;}
 static invalid():void {try {Probe.view(new Other());}catch(e){throw new Error("JS catch swallowed a runtime refusal");}}
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
	main := fmt.Sprintf(`defer func(){r:=recover();if _,ok:=r.(rangeFault);!ok{panic("invalid view was not a trap")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	requireCheckedClassViewABAP(t, p)
}
