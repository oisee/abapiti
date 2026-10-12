package tsfront

import (
	"fmt"
	"testing"
)

func TestErasedInPresenceBoundaryNodeOracle(t *testing.T) {
	source := `interface Shape {required:string|undefined;optional?:string;}
 class Native {value=1;method():void{}}
 class Probe {
 static has(x:object,key:string):boolean {return key in x;}
 static tagged():string {const a:Shape=JSON.parse('{"required":"r","extra":0}');const before=Probe.has(a,"optional");a.optional=undefined;return before+":"+Probe.has(a,"optional")+":"+Probe.has(a,"extra")+":"+Probe.has(a,"toString");}
 static run():string {const a:Shape={required:undefined};const b:Shape={required:undefined,optional:undefined};const before=Probe.has(a,"optional");a.optional=undefined;return Probe.has(a,"required")+":"+before+":"+Probe.has(b,"optional")+":"+Probe.has(a,"toString")+":"+Probe.has(a,"optional")+"/"+Probe.tagged();}
 static invalid():void {Probe.has(new Native(),"method");}
 }`
	want := conversionOracle(t, source)
	if want != "true:false:true:true:true/false:true:true:true\n" {
		t.Fatal(want)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok{panic("native prototype storage escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))); got != "refused\n" {
		t.Fatal(got)
	}
}

func TestDataInterfaceSubtypeNodeOracle(t *testing.T) {
	source := `interface Position {line:number;}
 interface Params {position:Position;uri:string;}
 interface RenameParams extends Params {newName:string;}
 class Probe {
 static view(p:Params):Params {p.uri="changed";p.position.line=9;return p;}
 static has(x:object,key:string):boolean {return key in x;}
 static run():string {const p:RenameParams={position:{line:1},uri:"old",newName:"new"};const view=Probe.view(p);p.uri="again";return (view===p)+":"+view.uri+":"+p.position.line+":"+p.newName+":"+Probe.has(view,"newName")+":"+Probe.has(view,"toString");}
 }`
	want := conversionOracle(t, source)
	for _, mode := range []string{"0", "1"} {
		t.Run("integer-mode-"+mode, func(t *testing.T) {
			t.Setenv("ABAPITI_ASSUME_INT", mode)
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
				t.Fatalf("Go %q Node %q", got, want)
			}
		})
	}
}

func TestOptionalChainFlowNodeOracle(t *testing.T) {
	source := `class Item {value=7;}
 interface Lookup {implementation?:Item;}
 class Probe {
 static take(x:Item):number{return x.value;}
 static test(lookup:Lookup|undefined):number {if(lookup?.implementation){return Probe.take(lookup?.implementation);}return -1;}
 static run():string {return Probe.test(undefined)+":"+Probe.test({})+":"+Probe.test({implementation:new Item()});}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestRecordFieldConstructorPresenceNodeOracle(t *testing.T) {
	source := `class Item {value=7;}
 interface Row {identifier:Item;name:string;}
 class Probe {
 static run():string {const vars:{[name:string]:Item}={a:new Item()};const rows:Row[]=[];for(const name in vars){if(vars[name].value===7){rows.push({name,identifier:vars[name]});}}return rows[0].name+":"+rows[0].identifier.value;}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestOptionalAssertionNodeOracle(t *testing.T) {
	source := `class Base {value=1;}
 class Child extends Base {extra=2;}
 class Probe {
 static calls=0;
 static once(x:Base|undefined):Base|undefined {Probe.calls++;return x;}
 static read(x:Base|undefined):number|undefined {const child=Probe.once(x) as Child|undefined;return child?.extra;}
 static present(x:Base):number|undefined {const child=x as Child|undefined;return child?.extra;}
 static run():string {return (Probe.read(undefined)===undefined)+":"+Probe.read(new Child())+":"+Probe.calls+":"+Probe.present(new Child());}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestCheckedPrimitiveFindNodeOracle(t *testing.T) {
	source := `class Probe {
 static run():string {
 const values=[0,1,2];let trace="";
 const found=values.find((v,i,a)=>{trace+=i+":"+v+":"+(a===values)+";";if(i===0){a[1]=9;a.push(3);}return v===9;});
 const zero=values.find(v=>v===0);const absent=values.find(v=>v===99);const empty:number[]=[];
 const none=empty.find(v=>true);const strings=["","x"];
 return found+"/"+(zero===undefined)+"/"+zero+"/"+(absent===undefined)+"/"+(none===undefined)+"/"+(strings.find(v=>v==="")===undefined)+"/"+trace;
 }
 static sparse():void {const a=["x"];a[2]="z";a.find(v=>v==="z");}
 }`
	want := conversionOracle(t, source)
	sparseNode := `class Probe {static run():string {const a=["x"];a[2]="z";let trace="";const found=a.find((v,i)=>{trace+=i+":"+(v===undefined)+";";return v==="z";});return found+"/"+trace;}}`
	if got := conversionOracle(t, sparseNode); got != "z/0:false;1:true;2:false;\n" {
		t.Fatal(got)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	// find visits holes as undefined in Node. This dense-only ABI must refuse
	// at Get before converting that undefined into an empty primitive.
	if got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok{panic("missing primitive slot escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "sparse"))); got != "refused\n" {
		t.Fatal(got)
	}
}
