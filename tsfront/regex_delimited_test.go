package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoRegExpDelimitedStringNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const p=/^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$/;
 const values:string[]=["", "'", "''", "''''", "'\\'", "'\\\\'", "'a\\'b'", "'a\\\\''", "'a\nb'", "'😀'", "'\\😀'", "'a\r'", "'\u2028\u2029'", "\"foo\""];
 let out="";for(const value of values){const hit=p.exec(value);out+=hit===null?"none;":hit[0]+":"+hit.index+";";}
 return out;
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
