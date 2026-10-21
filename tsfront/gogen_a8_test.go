package tsfront

import (
	"strings"
	"testing"
)

func TestGogenA8LiteralMember(t *testing.T) {
	gogenTSOracle(t, `type LiteralValue = string | number | {readonly [name:string]: LiteralValue | undefined};
interface Holder {value?:LiteralValue;}
function read(h:Holder):string { const v=h.value; if(typeof v === "string") return "s:"+v; if(typeof v === "number") return "n:"+v; if(v === undefined) return "u"; return Object.keys(v).join(",")+":"+JSON.stringify(v)+":"+("missing" in v)+":"+typeof v.x; }
class Probe {static run():string {return read({value:"hi"})+";"+read({value:2})+";"+read({})+";"+read({value:{z:1,x:{a:"ok"},missing:undefined,"2":"two"}});}}`, "")
}
func TestGogenA8MapMember(t *testing.T) {
	for _, kind := range []string{"Map", "ReadonlyMap"} {
		t.Run(kind, func(t *testing.T) {
			gogenTSOracle(t, `type StructDefinitions = `+kind+`<string,{name:string}>; interface Holder {structs?:StructDefinitions;} class Probe {static read(h:Holder):string {return h.structs ? h.structs.get("s")!.name+":"+h.structs.size : "absent";} static run():string {const m=new Map<string,{name:string}>(); m.set("s",{name:"ok"});return Probe.read({structs:m})+":"+Probe.read({});}}`, "")
		})
	}
}
func TestGogenA8WalkMember(t *testing.T) {
	gogenTSOracle(t, `interface WalkNode {[key:string]:unknown;name:string;count?:number;} function read(n:WalkNode,k:string):string {return n.name+":"+typeof n[k]+":"+typeof n["extra"]+":"+typeof n.extra+":"+JSON.stringify(n)+":"+Object.keys(n).join(",");} class Probe {static run():string {return read({name:"ok",extra:3},"extra")+";"+read({name:"n"},"absent");}}`, "")
}
func TestGogenA8MixedMember(t *testing.T) {
	gogenTSOracle(t, `type LiteralValue = string | number | {readonly [name:string]:LiteralValue|undefined}; type Expr = {e:"num",value:number} | {e:"wrap",child:Expr}; interface Holder {value?:Expr|LiteralValue;}
function read(h:Holder):string {const v=h.value; if(typeof v === "string") return v; if(typeof v === "number") return "n"+v; if(v === undefined) return "u"; if(v.e === "num") return "num:"+v.value; if(v.e === "wrap") return "wrap:"+JSON.stringify(v.child); return Object.keys(v).join(",")+":"+Object.entries(v).map(([k,x])=>k+"="+typeof x).join(",")+":"+JSON.stringify(v);}
class Probe {static run():string {return read({value:{e:"wrap",child:{e:"num",value:7}}})+";"+read({value:{x:undefined,z:"ok"}})+";"+read({value:3})+";"+read({});}}`, "")
}
func TestGogenA8MapIntersectionMember(t *testing.T) {
	gogenTSOracle(t, `type StructType={k:"struct";name:string}; type StructDefinitions=Map<string,StructType & {readonly fields:readonly {name:string}[]}>; interface Layers {structs?:StructDefinitions;} class Probe {static read(p:Layers):string {return p.structs ? p.structs.get("s")!.fields[0].name : "none";} static run():string {const m:StructDefinitions=new Map();m.set("s",{k:"struct",name:"S",fields:[{name:"ok"}]});return Probe.read({structs:m});}}`, "")
}
func TestGogenA8IndexWriteRefusal(t *testing.T) {
	for _, write := range []string{`n[k]=3`, `n[k]+=3`, `n[k]++`, `n.__proto__=null`} {
		_, ds := lowerProbe(t, sourceProbe(t, `interface WalkNode {[key:string]:unknown;name:string;} function f(n:WalkNode,k:string):void {`+write+`;}`))
		found := false
		for _, d := range ds {
			if d.Category == "unsupported-assignment" && strings.Contains(d.Message, "mixed index signature") && strings.Contains(d.Loc, "input.ts:") {
				found = true
			}
		}
		if !found {
			t.Errorf("missing located index write refusal for %s: %v", write, ds)
		}
	}
}

func TestGogenA8FamilyLiteralRoundTrip(t *testing.T) {
	gogenABAPFamilyOracle(t, `type Literal=string|number|{readonly [k:string]:Literal|undefined}; type Expr={e:"num",value:number}|{e:"wrap",child:Expr}; interface Holder {value?:Expr|Literal;} function describe(e:Expr):string {return e.e==="num" ? "n"+e.value : "w"+describe(e.child);} function pack(e:Expr):Holder {return {value:e};} class Probe {static run():string {const e:Expr={e:"wrap",child:{e:"num",value:4}};const h=pack(e);const v=h.value;return describe(e)+":"+JSON.stringify(v)+":"+typeof v+":"+(v!==undefined && typeof v==="object" && "e" in v);}}`)
}

func TestGogenA8IndexNumberAndEvaluation(t *testing.T) {
	gogenTSOracle(t, `interface WalkNode {[key:string]:unknown;name:string;} class Probe {static run():string {let n:WalkNode={name:"old","2":"two"};function key():string {n={name:"new","2":"changed"};return "2";} return typeof n[2]+":"+n[key()]+":"+n.name;}}`, "")
}

func TestGogenA8MemberPresence(t *testing.T) {
	gogenTSOracle(t, `type Value=string|number|{readonly [k:string]:Value|undefined};interface Holder {value?:Value;} function read(h:Holder):string {return Object.keys(h).join(",")+":"+JSON.stringify(h)+":"+typeof h.value;} class Probe {static run():string {return read({})+";"+read({value:undefined})+";"+read({value:{a:undefined,b:2}});}}`, "")
}

func TestGogenA8RecursiveIndexedNamedMember(t *testing.T) {
	gogenTSOracle(t, `interface WalkNode {[key:string]:unknown;name:string;child?:WalkNode;} function read(n:WalkNode):string {return n.child ? n.name+":"+read(n.child) : n.name;} class Probe {static run():string {const n:WalkNode={name:"a",child:{name:"b",extra:3}};return read(n)+":"+typeof n.child!.extra;}}`, "")
}
