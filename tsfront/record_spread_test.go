package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestPlainRecordSpreadNodeOracle(t *testing.T) {
	source := `interface Row {name:string; count:number; extra?:string;}
class Probe {
 static trace="";
 static record():Record<string,string|undefined> {Probe.trace+="r";return {z:"Z",name:"first",extra:undefined,"10":"ten","2":"two"};}
 static maybe(flag:boolean):Record<string,string|undefined>|undefined {return flag ? {extra:undefined,z:"optional"} : undefined;}
 static last():string {Probe.trace+="v";return "last";}
 static run():string {
 const a=Probe.record();const b:Record<string,string|undefined>={name:"second",q:"Q"};
 const both:Record<string,string|undefined>={...a,...b};const typed:Row={...{name:"typed",count:7},...a};
 const merged={...a,...b,name:Probe.last()};
 const absent={...Probe.maybe(false),name:"absent"};const optional={...Probe.maybe(true),name:"present"};a.name="changed";
 const row:Row={name:"row",count:2};const copy:Row={...row,count:3};
 const present:Row={name:"p",count:4,extra:undefined};const copied={...present};
 return Object.keys(both).join(",")+":"+(both.extra===undefined)+"|"+typed.name+"|"+Probe.trace+"|"+Object.keys(merged).join(",")+"|"+Object.keys(absent).join(",")+"|"+Object.keys(optional).join(",")+"|"+merged.name+"|"+Object.keys(copy).join(",")+"|"+copy.count+"|"+Object.keys(copied).join(",")+"|"+(copied.extra===undefined);
 }} `
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		if !strings.Contains(err.Error(), "probe.ts:") {
			t.Fatalf("unlocated ABAP refusal: %v", err)
		}
		t.Logf("located ABAP refusal: %v", err)
	} else {
		exportLiveViewABAP(t, p, "plain-record-spread", strings.TrimSpace(want), false)
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestPlainShapeSpreadUnionContextNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Shape {name:string;body?:string[];reason?:string;}
 class Probe {static run():string {const typed=new Map<string,Shape>();typed.set("m",{name:"m",body:[]});const rows:Shape[]=[];rows.push({...typed.get("m"),reason:"calls x"});let key; if(rows.length) key={line:true,type:"number"};else key={name:"n",type:"string"};const keys=[];keys.push({...key,desc:false});return JSON.stringify(rows)+"|"+JSON.stringify(keys);}}`, "own-property order/toJSON")
}

func TestPlainShapeSpreadInferredFamilyNodeOracle(t *testing.T) {
	gogenTSOracle(t, `type Query=({kind:"a";name:string}|{kind:"b";count:number}) & {role?:string};
 function merge(q:Partial<Query>):string {const out={role:"r",...q};return JSON.stringify(out)+":"+Object.keys(out).join(",");}
 class Probe {static run():string {return merge({kind:"a",name:"n"})+"|"+merge({kind:"b",count:2});}}`, "own-property order/toJSON")
}

func TestPlainRecordSpreadKeepsOperandRefusal(t *testing.T) {
	source := `interface Row {name:string;}class Probe {static run(row:Row):any {return {...row.missing};}}`
	root := false
	for _, d := range lowerFamilyDiagnostics(t, source) {
		if strings.Contains(d.Message, "tagged record spread needs object storage") {
			t.Fatalf("spurious storage refusal: %v", d)
		}
		root = root || strings.Contains(d.Message, "property missing")
	}
	if !root {
		t.Fatal("missing located operand refusal")
	}
}
