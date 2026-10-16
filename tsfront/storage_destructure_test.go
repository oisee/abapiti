package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestStorageDestructureNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const rows:any=JSON.parse('[["a",1],["b",2],[]]'); let out="";
 for(const [name, value] of rows) {out+=String(name)+":"+String(value)+";";}
 const pair:any=["x",false]; const [key,flag]=pair;
 const nums:any=[3,4]; const [first,second]=nums; return out+String(key)+":"+String(flag)+":"+String(first)+String(second);
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageOracleABAP(t, p, string(want))
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStorageDestructureRefusesComplexBindings(t *testing.T) {
	for _, pattern := range []string{"[a=1]", "[...a]", "[[a]]"} {
		t.Run(pattern, func(t *testing.T) {
			requireDiagnostic(t, sourceProbe(t, `class Probe {static run(value:any):void {const `+pattern+`=value;}}`), "unsupported-binding")
		})
	}
}
