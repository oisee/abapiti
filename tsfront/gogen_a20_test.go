package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func a20Oracle(t *testing.T, source string) {
	t.Helper()
	want := nodePlatformOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	_, _, err := emitABAPNamed(t, p)
	if err != nil {
		if !strings.Contains(err.Error(), "probe.ts:") {
			t.Fatal(err)
		}
		t.Logf("located ABAP refusal: %v", err)
		return
	}
}

func TestGogenA20CommaNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static log="";static hit(s:string):number {Probe.log+=s;return Probe.log.length;}
 static run():string {const n=(Probe.hit("a"),(Probe.hit("b"),Probe.hit("c"))); const yes=false&&(Probe.hit("x"),true); return n+":"+Probe.log+":"+yes;}}`)
}

func TestGogenA20VoidNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static log="";static hit():number {Probe.log+="v";return 9;}
 static run():string {const x=void Probe.hit();void Probe.hit();const y:number|undefined=void Probe.hit();return String(x===undefined)+":"+String(y===undefined)+":"+Probe.log;}}`)
}

func TestGogenA20ArrayAssignmentNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static calls=0;static values():number[]{Probe.calls++;return [7,8];}
 static run():string {let a=1,b=2;[a,b]=[b,a];const out=[a,b];[a,b]=Probe.values();out.push(a,b);const arr=[4,5];[arr[1],arr[0]]=arr;out.push(arr[0],arr[1]);let c:number|undefined=9;[,c]=[3];out.push(Probe.calls);return out.join(",")+":"+String(c===undefined);}}`)
}

func TestGogenA20CallResultNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static calls=0;static get():(n:number)=>string {Probe.calls++;return (n:number)=>"v"+n;}
 static run():string {return Probe.get()(3)+":"+Probe.calls;}}`)
}

func TestGogenA20ExhaustiveTagNodeOracle(t *testing.T) {
	a20Oracle(t, `type E={e:"num";value:number}|{e:"add";left:E;right:E};
 class Probe {static eval(x:E):string {switch(x.e){case "num":return String(x.value);case "add":return Probe.eval(x.left)+Probe.eval(x.right);default:throw new Error("tag "+x.e);}}
 static run():string {return Probe.eval({e:"add",left:{e:"num",value:1},right:{e:"num",value:2}});}}`)
}

func TestGogenA20ObjectCursorNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Cursor {idxVar?:string} interface Row {token:object}
 class Probe {static run():string {const row:Row={token:{}};const alias=row.token;row.token.idxVar="i1";return (alias as Cursor).idxVar??"missing";}}`)
}

func TestGogenA20MissingArrayAssignmentNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static run():string {let a=1,b=2;[a,b]=[3];return String(a)+":"+String(b===undefined);}}`)
}

func TestGogenA20ArrayAssignmentReferenceNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static log="";static a=[1,2];static target():number[]{Probe.log+="t";Probe.a[0]=7;return Probe.a;}static index():number{Probe.log+="i";return 1;}
 static run():string {const result=([Probe.target()[Probe.index()]]=Probe.a);return result.join(",")+":"+Probe.log;}}`)
}

func TestGogenA20OptionalFamilyPropertyNodeOracle(t *testing.T) {
	a20Oracle(t, `type E={e:"num";value:number}|{e:"add";left:E;right:E};interface Arg {value?:E}
 class Probe {static show(a:Arg):string {return a.value.e;}static run():string{return Probe.show({value:{e:"num",value:3}});}}`)
}

func TestGogenA20NestedFamilyPropertyNodeOracle(t *testing.T) {
	a20Oracle(t, `type T={k:"str"}|{k:"chars";len:number}|{k:"arr";item:T};type E={e:"num";type:T}|{e:"conv";to:T;x:E};
 class Probe {static show(e:E):string {if(e.e==="conv")return e.to.k==="chars"?String(e.to.len):"other";return e.type.k;}
 static run():string{return Probe.show({e:"conv",to:{k:"chars",len:3},x:{e:"num",type:{k:"str"}}});}}`)
}

func TestGogenA20ArrayIteratorExhaustionNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Box{x?:number} class Probe {static a:number[]=[];static b:Box={};static grow():Box {Probe.a.push(4,5);return Probe.b;}
 static run():string {let first:number|undefined=1;[first,Probe.grow().x]=Probe.a;return String(first===undefined)+":"+String(Probe.b.x===undefined)+":"+Probe.a.join(",");}}`)
}

func TestGogenA20MappedNestedFamilyNodeOracle(t *testing.T) {
	a20Oracle(t, `type T={name?:string}&({k:"str"}|{k:"chars";len:number}|{k:"arr";item:T});type E={e:"num";type:T}|{e:"conv";to:T;x:E};
 type Value<U>=U extends E?Expr:U extends readonly (infer V)[]?readonly Value<V>[]:U extends object?Fields<U>:U;type Fields<U>=U extends object?{readonly[P in keyof U]:Value<U[P]>}:U;type Expr=Fields<E>;
 class Probe {static kind(t:T):string{return t.k;}static show(e:Expr):string {if(e.e==="conv")return e.to.k==="chars"?String(e.to.len):Probe.kind(e.to);return Probe.kind(e.type);}
 static run():string{return Probe.show({e:"conv",to:{k:"chars",len:3},x:{e:"num",type:{k:"str"}}});}}`)
}

func TestGogenA20SiblingFamilyPayloadNodeOracle(t *testing.T) {
	a20Oracle(t, `type A={k:"leaf";len:number}|{k:"node";children:A[]};type B={k:"leaf";len:number}|{k:"node";children:B[]};
 class Probe {static first(x:A):number{return x.k==="leaf"?x.len:0;}static second(x:B):number{return x.k==="leaf"?x.len:0;}
 static run():string{return Probe.first({k:"leaf",len:2})+":"+Probe.second({k:"leaf",len:3});}}`)
}

func TestGogenA20CollectionFamilyFieldNodeOracle(t *testing.T) {
	a20Oracle(t, `type T={k:"chars";len:number}|{k:"arr";item:T};class Probe {static make():T{return {k:"chars",len:3};}
 static run():string {const rows=[{to:Probe.make()}];return rows.map(e=>e.to.k==="chars"?String(e.to.len):"other").join(",");}}`)
}

func TestGogenA20AbsentSwapNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static run():string {let a=1,b=2;[a,b]=[3];const result=([a,b]=[b,a]);return String(a===undefined)+":"+String(b)+":"+String(result[0])+":"+String(result[1]);}}`)
}

func TestGogenA20ObjectPropertyNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Cursor {idxVar?:string} interface Row {token:object} class Probe {static run():string {const row:Row={token:{}};row.token.idxVar="i1";const a:object=row.token;return String(a.idxVar)+":"+String(a.missing===undefined);}}`)
}
