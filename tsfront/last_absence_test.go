package tsfront

import (
	"fmt"
	"testing"
)

func TestAbsenceConstructorGuardNodeOracle(t *testing.T) {
	source := `class Message {private value:string;constructor(value:string){this.value=value;if(this.value===undefined){this.value="";}}read():string{return this.value;}}
 class Probe {static run():string {const input=JSON.parse('{}');const msg=new Message(input.number);return msg.read()+":"+(input.number===undefined);}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAbsenceNamedRowNodeOracle(t *testing.T) {
	source := `type Row={name:string,type:string};class Probe {static read(input:any):Row{return {name:input.NAME,type:input.TYPE};}static run():string {const input=JSON.parse('{"TYPE":"OKCODE"}');const row=Probe.read(input);const raw:any=row;return (raw.name===undefined)+":"+Object.keys(raw).join(",")+":"+raw.type;}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAbsenceLiveBooleanNodeOracle(t *testing.T) {
	source := `class Conf {enabled:boolean=true;}class Rule {conf=new Conf();setConfig(c:Conf):void{this.conf=c;}run():string {if(this.conf.enabled){return "yes";}return "no";}}class Probe {static run():string {const input=JSON.parse('{}');const rule=new Rule();rule.setConfig(input);const before=rule.run();input.enabled=true;return before+":"+rule.run()+":"+(rule.conf===input)+":"+Object.keys(input).join(",");}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAbsenceCommonIndexedAssertionNodeOracle(t *testing.T) {
	source := `interface Obj {name():string;}abstract class Base implements Obj {name():string{return "base";}}class Child extends Base {extra=2;}class Sibling extends Base {}class Consumer {obj:Base;constructor(obj:Base){this.obj=obj;}read():string{return this.obj.name();}}class Probe {static first(xs:Obj[]):string {const view=xs[0] as Child;return new Consumer(view).read();}static run():string {return Probe.first([new Sibling()]);}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAbsenceSiblingMethodDispatchNodeOracle(t *testing.T) {
	source := `interface Obj {name():string;}abstract class Base implements Obj {name():string{return "base";}}class Domain extends Base {parse(reg:string,extra?:string):string{return "domain";}}class DataItem extends Base {parse(reg:string):string{return "element:"+reg;}}class Probe {static read(input:Obj):string {const view=input as Domain;return view.parse("reg");}static run():string {return Probe.read(new DataItem());}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAbsenceNumericCoercionNodeOracle(t *testing.T) {
	source := `class Probe {static seen="";static left():number {Probe.seen=Probe.seen+"L";return 1;}static right():number|undefined {Probe.seen=Probe.seen+"R";return undefined;}static calls=0;static once(x?:number):number|undefined {Probe.calls=Probe.calls+1;return x;}static run():string {const rows:string[]=[];const value=rows[-1]?.length+1;const other=Probe.once(undefined)+Probe.once(2);const ordered=Probe.left()+Probe.right();return (value!==value)+":"+(other!==other)+":"+Probe.calls+":"+(Probe.once(3)+1)+":"+Probe.seen+":"+(ordered!==ordered);}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAbsenceBooleanKeepsStringLayoutsNodeOracle(t *testing.T) {
	source := `type Rows={name:string}[];class Probe {static rows():Rows {const ret=[];ret.push({name:"x"});return ret;}static run():string {return Probe.rows()[0].name||"missing";}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
