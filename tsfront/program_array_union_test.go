package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestProgramArrayUnionNodeOracle(t *testing.T) {
	source := `interface Item {name:string;}
interface Program {classes:readonly Item[];label?:string;}
type ClassList=Item[]&Partial<Program>;
type ProgramInput=Program|ClassList;
function read(program:ProgramInput):string {const classes=(program.classes??program) as readonly Item[];return (program.label??'none')+':'+classes.map(c=>c.name).join(',');}
class Probe {static run():string {
 const classes:ClassList=[{name:'a'},{name:'b'}];
 const program:Program={classes,label:'wrapped'};
 return read(classes)+'|'+read(program);
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
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestProgramArrayMetadataWriteLocatedRefusal(t *testing.T) {
	source := `interface Program {classes:string[];label?:string} type List=string[]&Partial<Program>;class Probe {static run():void {const list:List=[];list.label='x';}}`
	ds := lowerFamilyDiagnostics(t, source)
	for _, d := range ds {
		if d.Message == "named metadata writes on native array intersections are not lowered" && d.Loc != "" {
			return
		}
	}
	t.Fatalf("missing located metadata write refusal: %v", ds)
}
