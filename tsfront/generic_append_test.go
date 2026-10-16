package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGenericAppendNodeOracle(t *testing.T) {
	source := `function append<T>(target:T[],...arrays:readonly (readonly T[])[]):void {for(const array of arrays) for(const value of array) target.push(value);}
class Row {value:string;constructor(value:string){this.value=value;}}
class Probe {static run():string {
 const ns:number[]=[1];const alias=ns;append(ns,[2,3],[],[4]);
 const source:number[]=[6];const later=():number[]=>{source.push(7);return [9]};append(ns,source,later());
 const ss:string[]=['a'];append(ss,['b'],['c']);
 const row=new Row('x');const rows:Row[]=[];append(rows,[row]);row.value='y';
 const arrays:number[][]=[ns];append(arrays,[[8]]);arrays[0].push(5);
 const holes=new Array<number|undefined>(1);const optional:(number|undefined)[]=[];append(optional,holes);
 return JSON.stringify(optional)+'|'+alias.join(',')+'|'+ss.join(',')+'|'+rows[0].value+'|'+arrays[1].join(',');
} static sparse():void {const xs=new Array<number>(1);const out:number[]=[];append(out,xs);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	refusal := runGoHIR(t, p, fmt.Sprintf(`defer func(){r:=recover();if r==nil {panic("missing sparse refusal")};fmt.Println(r)}();%s()`, goEntry("probe.ts.Probe", "sparse")))
	if !strings.Contains(refusal, "probe.ts:") || !strings.Contains(refusal, "cannot represent undefined from holes") {
		t.Fatal(refusal)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
