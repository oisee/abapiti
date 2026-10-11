package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestGoGroupedSwitchLocalRangesNodeOracle(t *testing.T) {
	source := `class Probe {
 static get(kind:string,length:string,decimals:string|undefined):number {
 switch(kind) {
 case "a":case "b":case "c": {const n=Math.ceil((parseInt(length,10)+1)/2);const d=decimals===undefined?0:parseInt(decimals,10);return n+d;}
 case "d": {const d=7;return d;}
 default:return -1;
 }}
 static run():string {return Probe.get("a","3",undefined)+":"+Probe.get("b","4","2")+":"+Probe.get("c","5","1")+":"+Probe.get("d","0",undefined)+":"+Probe.get("z","0",undefined);}
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
	// Math.ceil is still an explicit ABAP refusal.
	if _, err := abap.Emit(p); err == nil {
		t.Fatal("expected ABAP ceil refusal")
	}
}
