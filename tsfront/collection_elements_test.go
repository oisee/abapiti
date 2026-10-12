package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestHeterogeneousCollectionElementsNodeOracle(t *testing.T) {
	source := `class Early { values=[new Left(),new Right()]; }
abstract class Expression { abstract name():string; }
class Left extends Expression { name():string{return "left";} }
class Right extends Expression { name():string{return "right";} }
class Third extends Expression { name():string{return "third";} }
interface Runnable {name():string;}
class Other implements Runnable {name():string{return "other";}}
class Another implements Runnable {name():string{return "another";}}
class Probe {
 values=[new Left(),new Right()];
 static run():string {
  const left=new Left();const right=new Right();
  const direct=[left,right];const reverse=[right,left];
  direct.push(new Third());
  const rows=[{value:left},{value:right}];
  const explicit:Expression[]=[left,right];
  const interfaces:Runnable[]=[left,new Other()];
  const shared=[new Other(),new Another()];
  const tuple:[Expression,Expression]=[left,right];
  const tupleRows=[[left,"a"] as const,[right,"b"] as const];
  const spread=[left,...[right]];
  const optional=[left,undefined,right];
  return direct.map(x=>x.name()).join(",")+"|"+reverse.map(x=>x.name()).join(",")+"|"+
   rows.map(x=>x.value.name()).join(",")+"|"+explicit.map(x=>x.name()).join(",")+"|"+
   interfaces.map(x=>x.name()).join(",")+"|"+shared.map(x=>x.name()).join(",")+"|"+
   tuple[0].name()+","+tuple[1].name()+"|"+(rows[1].value===right)+"|"+
   tupleRows.map(x=>x[0].name()+x[1]).join(",")+"|"+spread.map(x=>x.name()).join(",")+"|"+
   new Probe().values.map(x=>x.name()).join(",")+"|"+
   optional.map(x=>x===undefined?"absent":x.name()).join(",")+"|"+
   new Early().values.map(x=>x.name()).join(",");
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if errs := hir.Verify(p); len(errs) != 0 {
		t.Fatal(errs)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
