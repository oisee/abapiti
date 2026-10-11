package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoRegExpSubsetNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const patterns: RegExp[]=[/^.?cx_.*$/i,/^\/.+\/cx_.*$/i,/^"! .*<[^/>]*><\//,/^"!.+$/,/^"! @parameter .+\|/];
 const inputs:string[]=["CX_root","zCx_A","/namespace/CX_ROOT","/K/Cx_test","\"! <p></p>","\"! @parameter foo|", "\"!","cx_a\n","cx_a\r","cx_a\u2028","cx_a\u2029","cx_😀"];
 let out="";for(const p of patterns){for(const s of inputs){const hit=p.exec(s);out+=hit===null?"none;":hit[0]+":"+hit.index+";";}}
 return out;
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
