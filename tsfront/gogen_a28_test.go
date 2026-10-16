package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func a28Oracle(t *testing.T, source string) {
	t.Helper()
	want := nodePlatformOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if t.Failed() {
		return
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, _, err := emitABAPNamed(t, p); err != nil {
		if !strings.Contains(err.Error(), "probe.ts:") {
			t.Fatal(err)
		}
		t.Logf("located ABAP refusal: %v", err)
	} else {
		exportLiveViewABAP(t, p, t.Name(), strings.TrimSpace(want), false)
	}
}

func TestGogenA28FamilyPresenceNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"field";base:E;name:string}|{e:"row";base:E;index:number};
 type T={k:"str"}|{k:"arr";item:T};
 class Probe {
 static show(n:E|T|boolean):string {if(n&&typeof n==="object") {if("base" in n)return n.base.e;if("e" in n)return n.e;if("k" in n)return n.k;}return "other";}
 static run():string {const leaf:E={e:"leaf",value:2};return [Probe.show(leaf),Probe.show({e:"field",base:leaf,name:"a"}),Probe.show({e:"row",base:leaf,index:1}),Probe.show({k:"arr",item:{k:"str"}}),Probe.show(true)].join("/");}}`)
}

func TestGogenA28SubsetFieldNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"field";base:E;name:string}|{e:"row";base:E;index:number};
 class Probe {static show(n:E):string {return "base" in n?n.base.e:n.e;}
 static run():string {const leaf:E={e:"leaf",value:2};return [Probe.show(leaf),Probe.show({e:"field",base:leaf,name:"a"}),Probe.show({e:"row",base:leaf,index:1})].join("/");}}`)
}

func TestGogenA28LocalFieldNodeOracle(t *testing.T) {
	a28Oracle(t, `type T={k:"str"}|{k:"table";row:T};interface Field {readonly name:string;readonly type:T} interface Local extends Field {readonly static?:boolean;readonly value?:string}
 class Probe {static owned(f:Field):string{return f.name+":"+f.type.k;}
 static run():string {const l:Local={name:"x",type:{k:"table",row:{k:"str"}},value:"extra"};return Probe.owned(l)+":"+JSON.stringify(l);}}`)
}

func TestGogenA28MapJoinNodeOracle(t *testing.T) {
	a28Oracle(t, `interface Row {readonly name:string;readonly args:readonly string[]} type Rows=Map<string,{readonly name:string;readonly args:readonly string[]}>;
 class Probe {static choose(flag:boolean,m:Map<string,Row>):Rows{return flag?new Map():m;}
 static run():string {const m=new Map<string,Row>();m.set("a",{name:"v",args:["x"]});const a=Probe.choose(false,m);a.set("b",{name:"B",args:[]});return a.get("a")!.name+":"+m.size+":"+(a===m)+":"+Probe.choose(true,m).size;}}`)
}

func TestGogenA28StoredCallableNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"link";base:E};type Move=(st:E,next:E|undefined)=>string;let move:Move|undefined;
 function pair(st:E,next:E|undefined):string{return st.e+":"+(next?.e??"missing");}
 class Probe {static run():string {move=pair;return move({e:"leaf",value:2},undefined);}}`)
}

func TestGogenA28ReplaceCallbackNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {let trace="";const code='a1 b22'.replace(/([a-z])(\d+)/g,(match:string,a:string,b:string,offset:number,input:string):string=>{trace+=match+":"+offset+":"+input+"/";return b+a;});return code+"|"+trace;}}`)
}

func TestGogenA28EmptyFallbackNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {const m:Map<string,string>|undefined=new Map<string,string>();m.set("x","y");return Array.from(m??[]).map(([k,v]):string=>k+v).join("/");}}`)
}

func TestGogenA28ObjectSortNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {const a=[{name:"z",items:["x"]},{name:"a",items:["y"]},{name:"m",items:[]}];const alias=a;const sorted=a.sort();return sorted.map(x=>x.name).join("/")+":"+(alias===sorted);}}`)
}

func TestGogenA28ReplaceStateNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {
 const re=/(a)(b)?/g;re.lastIndex=7;let seen="";
 const a="a ab".replace(re,(match:string,x:string,y:string|undefined,offset:number,input:string):string=>{seen+=match+":"+String(y)+":"+offset+":"+re.lastIndex+":"+input+"/";re.lastIndex=9;return "$"+x;});
 const single=/a/;single.lastIndex=3;const b="aa".replace(single,(m:string,offset:number,input:string):string=>offset+input);
 const c="ab".replace(/(?:)/g,(m:string,offset:number):string=>String(offset));
 const d="ab a".replace(/(?<x>a)(?<y>b)?/g,(m:string,x:string,y:string|undefined,offset:number,input:string,groups:any):string=>groups.x+":"+String(groups.y)+":"+offset);
 return a+"|"+seen+"|"+re.lastIndex+"|"+b+":"+single.lastIndex+"|"+c+"|"+d;
 }}`)
}

func TestGogenA28DynamicFamilyJoinNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"link";base:E};
 function isExpr(n:any):n is E{return !!n&&typeof n==="object"&&"e" in n;}
 class Probe {static choose(n:any,flag:boolean):E|undefined {return flag&&isExpr(n)?n:undefined;}
 static run():string {const n:any={e:"link",base:{e:"leaf",value:7}};const a=Probe.choose(n,true);return (a?.e??"missing")+":"+(a===n)+":"+String(Probe.choose(n,false)===undefined);}}`)
}

func TestGogenA28ConcatAbsentFieldNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"link";base:E};type S={s:"concat";parts:E[];table?:undefined}|{s:"concat";table:E;parts?:undefined}|{s:"seq";body:S[]};type C=Extract<S,{s:"concat"}>;
 class Probe {static show(n:C):string {if(n.table)return n.table.e;return n.parts.map(x=>x.e).join("/");}
 static run():string {const a:any={s:"concat",parts:[{e:"leaf",value:2}]};const b:any={s:"concat",table:{e:"leaf",value:4}};return Probe.show(a)+":"+Probe.show(b);}}`)
}

func TestGogenA28ConsoleErrorNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {console.error('trace %s: 😀\nsecond');return 'done';}}`)
}

func TestGogenA28OptionalMapJoinNodeOracle(t *testing.T) {
	a28Oracle(t, `interface Row {readonly name:string;readonly args:readonly string[]} type Rows=Map<string,{readonly name:string;readonly args:readonly string[]}>;interface Input {rows:Map<string,Row>}
 class Probe {static choose(flag:boolean,input:Input|Row[]):Rows{return Array.isArray(input)?new Map():input.rows;}
 static run():string {const m=new Map<string,Row>();m.set("a",{name:"v",args:["x"]});return Probe.choose(false,{rows:m}).get("a")!.name+":"+Probe.choose(true,[]).size;}}`)
}

func TestGogenA28EntrySortNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {const m=new Map<string,{name:string}[]>();m.set("a",[{name:"z"},{name:"a"}]);m.set("a,",[]);m.set("b",[{name:"b"}]);const rows=[...m].sort();return rows.map(([k,v]):string=>k+":"+v.map(x=>x.name).join("/")).join("|");}}`)
}

func TestGogenA28CapturedCallableNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"link";base:E};type Move=(st:E,next:E|undefined)=>string;
 function pair(st:E,next:E|undefined):string|false{return st.e+":"+(next?.e??"missing");}
 function analyze():Move{return (st,next):string=>{const rule=pair(st,next);return String(rule);};}
 class Probe {static run():string {const f=analyze();return f({e:"leaf",value:3},undefined);}}`)
}

func TestGogenA28MapViewNodeOracle(t *testing.T) {
	a28Oracle(t, `interface A {readonly name:string;readonly args:readonly string[]} interface B {readonly args:readonly string[];readonly name:string}
 class Probe {static choose(flag:boolean,a:Map<string,A>,b:Map<string,B>):Map<string,A>{return flag?a:b;}
 static run():string {const a=new Map<string,A>(),b=new Map<string,B>();const row:B={args:["x"],name:"B"};b.set("one",row);const v=Probe.choose(false,a,b);v.set("two",{name:"T",args:[]});b.set("three",{args:["y"],name:"R"});const first=v.get("one");v.delete("one");v.set("one",{name:"N",args:[]});return (v===b)+":"+(first===row)+":"+v.size+":"+b.get("two")!.name+":"+[...v.keys()].join("/")+":"+[...v.values()].map(r=>r.name).join("/")+":"+String(v.get("missing")===undefined);}}`)
}

func TestGogenA28MapViewKeysNodeOracle(t *testing.T) {
	a28Oracle(t, `interface A {readonly name:string;readonly args:readonly string[]} interface B {readonly args:readonly string[];readonly name:string}
 class Probe {static choose(a:Map<string,A>,b:Map<string,B>):Map<string,A>{return b;}
 static run():string {const a=new Map<string,A>(),b=new Map<string,B>();b.set("row",{name:"R",args:[]});const v=Probe.choose(a,b);const x=v.get("row")!,y=v.get("row")!;const m=new Map<A,string>();m.set(x,"live");const s=new Set<A>();s.add(x);s.add(y);return m.get(y)+":"+s.size+":"+m.has(y)+":"+m.delete(y)+":"+m.size;}}`)
}

func TestGogenA28ForOfFallbackNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static show(m:Map<string,string>|undefined):string {const out:string[]=[];for(const [k,v] of m??[])out.push(k+v);return out.join("/");}static run():string {const m=new Map<string,string>();m.set("a","b");return Probe.show(m)+":"+Probe.show(undefined);}}`)
}

func TestGogenA28FlatMapTuplesNodeOracle(t *testing.T) {
	a28Oracle(t, `interface M {name:string} interface C {name:string;methods?:M[];ctor?:M}
 class Probe {static run():string {const classes:C[]=[{name:"A",methods:[{name:"m"}],ctor:{name:"c"}},{name:"B"}];const bodies:[C,M][]=[...classes.flatMap((cls):[C,M][]=>[...(cls.methods??[]),...(cls.ctor?[cls.ctor]:[])].map((method):[C,M]=>[cls,method]))];return bodies.map(([c,m]):string=>c.name+":"+m.name).join("/");}}`)
}

func TestGogenA28FamilyTypeofNodeOracle(t *testing.T) {
	a28Oracle(t, `type E={e:"leaf";value:number}|{e:"link";base:E};class Probe {static tag(n:E):string{return typeof n;}static run():string{return Probe.tag({e:"leaf",value:1});}}`)
}

func TestGogenA28LocalPresenceStorageNodeOracle(t *testing.T) {
	a28Oracle(t, `type T={k:"str"}|{k:"table";row:T};interface Field {readonly name:string;readonly type:T} interface Local extends Field {readonly extra?:string}
 class Probe {static missing():any{return undefined;}static inspect(f:Field):string{return f.type.k;}static run():string {const l:Local={name:Probe.missing(),type:{k:"str"}};return Probe.inspect(l)+":"+String(l.name===undefined);}}`)
}

func TestGogenA28NestedMapViewNodeOracle(t *testing.T) {
	a28Oracle(t, `interface Left {readonly name:string;readonly value:number} interface Right {readonly value:number;readonly name:string} interface A {readonly args:readonly Left[]} interface B {readonly args:readonly Right[]}
 class Probe {static choose(a:Map<string,A>,b:Map<string,B>):Map<string,A>{return true?b:a;}static run():string {const a=new Map<string,A>(),b=new Map<string,B>();const rows:Right[]=[{value:1,name:"one"}];b.set("r",{args:rows});const v=Probe.choose(a,b);rows.push({value:2,name:"two"});return v.get("r")!.args.map(x=>x.name).join("/")+":"+(v===b);}}`)
}

func TestGogenA28ReplaceConstantNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {return 'a'.replace(/a/,()=> 'x');}}`)
}

func TestGogenA28CollectionMapFnRefusal(t *testing.T) {
	source := `class Probe {static show(m:Map<string,string>|undefined):string[] {return Array.from(m??[],([k,v]):string=>{m!.set("new","entry");return k+v;});}}`
	for _, d := range lowerFamilyDiagnostics(t, source) {
		if strings.Contains(d.Message, "live collection iterator") && strings.HasPrefix(d.Loc, "probe.ts:") {
			return
		}
	}
	t.Fatal("collection callback snapshot was accepted without a live iterator")
}

func TestGogenA28FreshMapMjsNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @typedef {import("./types.ts").Item} Item */
/** @typedef {{structs:Map<string,Item>}} Input */
/** @param {Input | Item[]} program @param {Input | undefined} layers @returns {Map<string,Item>} */
function choose(program,layers) {const structs=layers?.structs ?? (Array.isArray(program) ? new Map() : program.structs);return Array.isArray(program)?structs:program.structs;}
export class Probe {
 /** @returns {string} */ static run() {
  /** @type {Map<string,Item>} */
  const m=new Map([['k',{name:'v',size:1}]]);
  return choose({structs:m},undefined).get('k').name+':'+choose([],undefined).size;
 }
}`, false)
}

func TestGogenA28LocalSetStorageNodeOracle(t *testing.T) {
	a28Oracle(t, `type T={k:"str"}|{k:"table";row:T};interface Field {readonly name:string;readonly type:T} interface Local extends Field {readonly extra?:string}
 class Probe {static missing():any{return undefined;}static run():string {const l:Local={name:Probe.missing(),type:{k:"str"}};const s=new Set<Field>();s.add(l);return String(s.has(l))+":"+s.size;}}`)
}

func TestGogenA28BindingScannerReplaceNodeOracle(t *testing.T) {
	a28Oracle(t, `class Probe {static run():string {const source='a/* one 😀 */b/* two */c // comment\r\n"quoted \\\" text" d\n\'single\' e\n`+"`template`"+`f 😀';return source.replace(/\/\/[^\n]*|\/\*[\s\S]*?\*\/|"(?:\\[\s\S]|[^"\\])*"|'(?:\\[\s\S]|[^'\\])*'|`+"`[^`]*`"+`/g,(text):string=>text.replace(/[^\n]/g,' '));}}`)
}
