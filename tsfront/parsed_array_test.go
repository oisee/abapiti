package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoParsedArrayIdentityNodeOracle(t *testing.T) {
	source := `class Probe {
 static toArray(data:any):any[] {if(data===undefined){return [];}else if(Array.isArray(data)){return data;}else{return [data];}}
 static run():string {
 const data:any=JSON.parse('["first",{"value":"kept"}]');
 const a=Probe.toArray(data);const b=Probe.toArray(data);
 a[0]="changed";b.push("added");
 const singleton=Probe.toArray(data[1]);
 return (a===b)+":"+(a===data)+":"+data.length+":"+data[0]+":"+singleton[0].value+":"+JSON.stringify(data)+":"+Probe.toArray(undefined).length;
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
