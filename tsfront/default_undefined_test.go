package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestDefaultUndefinedNodeOracle(t *testing.T) {
	source := `class Probe {
 static calls=0;
 static fallback():string {Probe.calls++;return "fallback";}
 static missing(value:string|undefined=undefined):string {return value===undefined?"absent":value;}
 static lazy(value:string|undefined=Probe.fallback()):string {return value===undefined?"absent":value;}
 static run():string {return [Probe.missing(),Probe.missing(undefined),Probe.missing(""),Probe.missing("given"),Probe.lazy("given"),Probe.lazy(),Probe.lazy(undefined)].join(":")+":"+Probe.calls;}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
