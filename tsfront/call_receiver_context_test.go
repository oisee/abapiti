package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestCallReceiverContextNodeOracle(t *testing.T) {
	// The return and argument contexts are number[], but each shift receiver is
	// number[][]. Its literal must push arrays, not numbers, into the outer array.
	source := `class Row {value=5;read():number[] {return [this.value];}}
 class Probe {
 static calls=0;
 static row(value:number):number[] {Probe.calls++;return [value];}
 static returned():number[] {return [Probe.row(1),Probe.row(2)].shift()!;}
 static virtual():number[] {return [new Row()].shift()!.read();}
 static consume(row:number[]):string {return row.join(",");}
 static run():string {
 const first=Probe.returned();
 const second=Probe.consume([Probe.row(3),Probe.row(4)].shift()!);
 return first.join(",")+":"+second+":"+Probe.virtual().join(",")+":"+Probe.calls;
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
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}
