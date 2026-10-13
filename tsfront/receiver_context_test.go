package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoArrayReceiverContextNodeOracle(t *testing.T) {
	source := `class Box {value:number; constructor(value:number) {this.value=value;}}
 class Probe {
 static calls=0;
 static row(n:number):Box[] {Probe.calls++;return [new Box(n)];}
 static run():string {
 const a:Box[]=[Probe.row(1),Probe.row(2),[]].flat();
 const b:Box[][]=[Probe.row(3),Probe.row(4)].reverse();
 a[0].value=7;
 return a[0].value+":"+a[1].value+":"+a.length+":"+b[0][0].value+":"+Probe.calls;
 }}`
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
