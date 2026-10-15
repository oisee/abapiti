package tsfront

import "testing"

func TestGogenA2GeneratorCall(t *testing.T) {
	gogenOracle(t, `interface IObject {name:string}
 class Registry {trace=""; *getObjects(): Generator<IObject> {this.trace+="a";yield {name:"one"};this.trace+="b";yield {name:"two"};this.trace+="c";throw new Error("end");}}
 class Probe {static run():string {const r=new Registry();let out="";try {for(const o of r.getObjects()){out+=o.name+r.trace;}}catch(e){out+=":"+r.trace;}const s=new Registry();try {const a=[...s.getObjects()];out+=""+a.length;}catch(e){out+=":"+s.trace;}return out;}}`, "oneatwoab:abc:abc")
}

func TestGogenA2LogicalValues(t *testing.T) {
	gogenOracle(t, `class Session {value=7;}
 function both(s:Session|undefined):boolean|undefined {return s && false;}
 function either(s:Session|undefined):Session|string {return s || "fallback";}
 class Probe {static run():string {return ""+both(undefined)+":"+both(new Session())+":"+either(undefined)+":"+(either(new Session()) instanceof Session);}}`, "undefined:false:fallback:true")
}

func TestGogenA2BindingDefaults(t *testing.T) {
	gogenOracle(t, `interface Opts {x?:number|null;y?:number;cache?:string;depth?:number;}
 function f(a:number,{x=1,y=2}:Opts={}):string {return a+":"+x+":"+y;}
 function g({cache=undefined,depth=0}:Opts):string {return ""+cache+":"+depth;}
 class Probe {static run():string {let out=f(3)+"/"+f(4,{x:null,y:0})+"/"+g({});try {f(0,null as any);}catch(e){out+="/null";}return out;}}`, "3:1:2/4:null:0/undefined:0/null", "RequireObjectCoercible")
}

func TestGogenA2NamedCallbacks(t *testing.T) {
	gogenOracle(t, `function predicate(v:number,i:number,a:number[]):boolean {return v+i===a.length;}
 function keyOf(v:number,i:number,a:number[]):string {return ""+v+":"+i+":"+a.length;}
 function apply(key:(v:number,i:number,a:number[])=>string):string {return [2,2,2].filter(predicate).map(key).join("/");}
 class Probe {static run():string {return apply(keyOf);}}`, "2:0:1")
}

func TestGogenA2FinallyReturn(t *testing.T) {
	gogenOracle(t, `class Probe {static value=0;static f():number {try {Probe.value+=1;return Probe.value;}finally {Probe.value+=10;}}static run():string {const v=Probe.f();return v+":"+Probe.value;}}`, "1:11")
}
func TestGogenA2FinallyThrow(t *testing.T) {
	gogenOracle(t, `class Probe {static trace="";static f():number {try {Probe.trace+="r";return 7;}finally {Probe.trace+="f";throw new Error("override");}}static run():string {try {Probe.f();}catch(e){return Probe.trace+":"+(e as Error).message;}return "bad";}}`, "rf:override")
}
func TestGogenA2TryThrowFinally(t *testing.T) {
	gogenOracle(t, `class Probe {static trace="";static f():void {try {Probe.trace+="t";throw new Error("original");}finally {Probe.trace+="f";}}static run():string {try {Probe.f();}catch(e){return Probe.trace+":"+(e as Error).message;}return "bad";}}`, "tf:original")
}

func TestGogenA2BindingOrder(t *testing.T) {
	gogenOracle(t, `class Source {static trace="";get x():number|undefined {Source.trace+="x";return undefined;}get y():number|undefined {Source.trace+="y";return undefined;}static def(n:number):number {Source.trace+=""+n;return n;}}
 function f({x=Source.def(1),y=Source.def(2)}:Source):string {return x+":"+y+":"+Source.trace;}
 class Probe {static run():string {return f(new Source());}}`, "1:2:x1y2", "RequireObjectCoercible")
}

func TestGogenA2LogicalLazy(t *testing.T) {
	gogenOracle(t, `class Session {value=7;}
 class Probe {static trace="";static arg(p:boolean):Session|undefined {Probe.trace+="l";return p?new Session():undefined;}static rhs():boolean {Probe.trace+="r";return false;}static fallback():string {Probe.trace+="f";return "fallback";}static run():string {const a=Probe.arg(false)&&Probe.rhs();const b=Probe.arg(true)&&Probe.rhs();const c=Probe.arg(true)||Probe.fallback();const d=Probe.arg(false)||Probe.fallback();return ""+a+":"+b+":"+(c instanceof Session)+":"+d+":"+Probe.trace;}}`, "undefined:false:true:fallback:llrllf")
}

func TestGogenA2NamedCallbackForEach(t *testing.T) {
	gogenOracle(t, `function record(v:number,i:number,a:number[]):void {Probe.trace+=""+v+i+a.length;}
 class Probe {static trace="";static run():string {const local=record;[4,5].forEach(record);[6].forEach(local);return Probe.trace;}}`, "402512601")
}

func TestGogenA2CallbackArityRefusal(t *testing.T) {
	requireDiagnostic(t, sourceProbe(t, `function key(v:number,i:number,a:number[],extra:number):number {return v;} class Probe {static run():number[] {return [1].map(key);}}`), "unsupported-callback")
}

func TestGogenA2FinallyConditional(t *testing.T) {
	gogenOracle(t, `class Probe {static trace="";static f(p:boolean):number {try {const v=3;if(p){Probe.trace+="r";return v;}Probe.trace+="n";}finally {Probe.trace+="f";}Probe.trace+="a";return 4;}static run():string {const x=Probe.f(true);const y=Probe.f(false);return x+":"+y+":"+Probe.trace;}}`, "3:4:rfnfa")
}

func TestGogenA2LocalArrowCallbacks(t *testing.T) {
	gogenOracle(t, `class Probe {static run():string {const offset=2;const predicate=(v:number):boolean=>v>offset;const key=(v:number,i:number,a:number[]):string=>""+v+i+a.length;return [1,3,4].filter(predicate).map(key).join("/");}}`, "302/412")
}

func TestGogenA2CallbackEvaluationOrder(t *testing.T) {
	gogenOracle(t, `function first(v:number):string {return "first"+v;}function second(v:number):string {return "second"+v;}
 class Probe {static run():string {let key:(v:number)=>string=first;const source=():number[]=>{key=second;return [1];};return source().map(key).join();}}`, "second1")
}

func TestGogenA2GeneratorSpreadValues(t *testing.T) {
	gogenOracle(t, `interface IObject {name:string} class Registry {trace="";*getObjects():Generator<IObject> {this.trace+="a";yield {name:"one"};this.trace+="b";yield {name:"two"};this.trace+="c";}}
 class Probe {static run():string {const r=new Registry();const g=r.getObjects();const before=r.trace;const a=[...g];return before+":"+a.map(o=>o.name).join(",")+":"+r.trace;}}`, ":one,two:abc")
}
