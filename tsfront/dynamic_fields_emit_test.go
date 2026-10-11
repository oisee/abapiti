package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestGoDynamicClassFieldsNodeOracle(t *testing.T) {
	source := `class Base { severity: string="error"; }
 class Conf extends Base {first: boolean=true;second:boolean=false;count:number=1;}
 class Probe {static run(): string {
 const c=new Conf();for(const key of Object.keys(c)) {if(typeof (c as any)[key]==="boolean"){(c as any)[key]=false;}}
 const k="second";(c as any)[k]=true;
 const rows: any[]=[{description:"one",optional:undefined},{description:"two",optional:3}];let titles="";
 rows.forEach((r:any)=>{if(r.description === "one"){titles += "one";}else if(r.description === "two"){titles += "two";}});
 return Object.keys(c).join(",")+":"+c.first+":"+c.second+":"+c.count+":"+titles;
 }} `
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source)
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	// The dynamic receiver helper is certified only in the real tests closure;
	// an array annotated any[] exercises the same erased-row property adapter.
	p := integerProbe(t, source)
	prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if es := hir.Verify(prog); len(es) > 0 {
		t.Fatal(es)
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
