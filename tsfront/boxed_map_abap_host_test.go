package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestBoxedConstructorRecordABAPHost(t *testing.T) {
	source := `class Item { name:string; constructor(name:string){this.name=name;} }
 class Other extends Item {}
 class Probe {
 static run():string {
 const typed:{[key:string]:typeof Item}={Item};
 const erased:any=typed;
 const alias:any=typed;
 const first=erased["Item"];
 typed["Other"]=Other;
 const made:Item=new erased["Other"]("made");
 erased["Item"]=Other;
 const changed=typed["Item"]===Other;
 return (first===Item)+":"+(alias["Other"]===Other)+":"+made.name+":"+(made instanceof Other)+":"+changed+":"+(erased["absent"]===undefined)+":"+(alias===erased);
 }
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestErrorSubclassCatchABAPHost(t *testing.T) {
	source := `class Special extends Error {}
 class Child extends Special {}
 class Probe {
 static run():string {
 const original=new Child("detail");
 let identity=false;
 let inherited=false;
 try {try {throw original;} catch(e) {identity=e===original;throw e;}}
 catch(e) {inherited=e instanceof Child && e instanceof Special && e instanceof Error;}
 return identity+":"+inherited;
 }
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
