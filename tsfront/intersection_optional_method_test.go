package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestIntersectionOptionalMethodNodeOracle(t *testing.T) {
	source := `interface ArtifactObject {getName():string;getFiles():readonly string[];isDirty?():boolean;}
class ABAPObject {getABAPFiles():readonly string[] {return ['a','b'];}}
type OwnershipObject=ArtifactObject & Partial<Pick<ABAPObject,'getABAPFiles'>>;
class Probe {static read(obj:OwnershipObject):string {return obj.getName()+':'+(obj.getABAPFiles?.()??[]).join(',');}
static run():string {const plain:OwnershipObject={getName:()=> 'plain',getFiles:():readonly string[]=> []};const rich:OwnershipObject={getName:()=> 'rich',getFiles:():readonly string[]=> [],getABAPFiles:()=> ['a','b']};return Probe.read(plain)+'|'+Probe.read(rich);}}
`
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
