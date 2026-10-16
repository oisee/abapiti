package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestStorageSortNodeOracle(t *testing.T) {
	source := `function compare(a:number,b:number):number {return (a<20?0:1)-(b<20?0:1);}
 function sorted(a:number[],cmp:(a:number,b:number)=>number):string {return a.sort(cmp).join();}
 class Probe {static compare(a:number,b:number):number{return compare(a,b);}
 static observe():string {const a=[3,2,1];a.sort((x,y)=>x-y+(a[0]-3)*10);return a.join();}
 static run():string {
 const by=(a:number,b:number):number=>compare(a,b);
 const a=[21,11,22,12,23]; const alias=a; const result=a.sort(by);
 const b=[21,11,22,12,23]; b.sort(Probe.compare);
 const c=[21,11,22,12,23]; c.sort(compare);
 const rows:any[]=JSON.parse('[["b",null],["a","z"],["a","y"]]'); rows.sort();
 const words=["z","a","😀","\ue000","a"]; words.sort();
 const pairs:[string,string][]=[["b","x"],["a","z"],["a","y"]]; pairs.sort();
 return Probe.observe()+":"+rows.map(r=>String(r[0])+"/"+String(r[1])).join(";")+":"+sorted([21,11,22,12,23],by)+":"+[2,10,1,-2].sort().join()+":"+a.join()+":"+b.join()+":"+c.join()+":"+(alias===result)+":"+words.join()+":"+pairs.map(p=>p[0]+"/"+p[1]).join(";");
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

func TestStorageSortRefusesUncertifiedConversion(t *testing.T) {
	requireDiagnostic(t, sourceProbe(t, `class Item {value=1;} class Probe {static run():void {[new Item()].sort();}}`), "unsupported-call")
}

func TestStorageSortThrowNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const a=[3,2,1,99];try{a.sort((x,y)=>{if(x===99||y===99)throw new Error("compare");return x-y;});}catch(e){return a.join()+":"+(e as Error).message;}return "bad";}}`
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
