package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestGogenA9ResolvedValue(t *testing.T) {
	gogenTSOracle(t, `type IRNode={e:"var";name:string;child?:IRValue}|{s:"block";body:readonly IRValue[]};
 type IRValue=IRNode|readonly IRValue[]|string|number|boolean|null|undefined;
 type Inspection<T>=T extends readonly (infer V)[] ? readonly Inspection<V>[] : T extends object ? {readonly [K in keyof T]?:Inspection<T[K]>}:T;
 type Value=Inspection<IRValue>;
 function inspect(v:Value):string {return typeof v+":"+JSON.stringify(v);}
 class Probe {static run():string {return inspect({e:"var",name:"x"})+";"+inspect([true,null,undefined,{s:"block",body:[]}])+";"+inspect(3);}}`, "")
}

func TestGogenA9AnonymousPositions(t *testing.T) {
	gogenTSOracle(t, `function read(x:{readonly name:string;ctorParams?:readonly {name:string}[]}):string {return x.name+":"+(x.ctorParams ? x.ctorParams[0].name : "none");}
 function make():{readonly name:string;ctorParams?:readonly {name:string}[]} {return {name:"r"};}
 class Holder {value:{readonly name:string;ctorParams?:readonly {name:string}[]}={name:"f"};}
 class Probe {static run():string {const a={name:"a"};return read(a)+";"+read(make())+";"+read(new Holder().value)+";"+read({name:"p",ctorParams:[{name:"c"}]});}}`, "")
}

func TestGogenA9LiteralSemantics(t *testing.T) {
	gogenTSOracle(t, `function read(x:{readonly name:string;extra?:string}):string {return Object.keys(x).join(",")+":"+("extra" in x)+":"+typeof x.extra+":"+JSON.stringify(x);}
 class Probe {static run():string {const a={name:"a"};const b={extra:undefined,name:"b","2":"two","1":"one"};return read(a)+";"+read(b)+";"+read({name:"c",extra:undefined});}}`, "")
}

func TestGogenA9GenericInstantiation(t *testing.T) {
	gogenTSOracle(t, `type Box<T>={readonly value:T};type Value=Box<string>;function read(x:Value):string{return x.value;} class Probe {static run():string {const a:Box<number>={value:2};return read({value:"ok"})+":"+a.value;}}`, "")
}

func TestGogenA9MappedObject(t *testing.T) {
	gogenTSOracle(t, `type Expr={e:"num";n:number}|{e:"wrap";child:Expr};interface Method {name:string;body:readonly Expr[];} interface Class {name:string;ctorParams?:readonly {name:string;type:Expr}[];methods:readonly Method[];} type Cache<T>=T extends readonly (infer V)[] ? readonly Cache<V>[] : T extends object ? {[K in keyof T]:Cache<T[K]>}:T; type ClassIR=Cache<Class>;function read(c:ClassIR):string {return c.name+":"+JSON.stringify(c);} class Probe {static run():string {return read({name:"C",methods:[{name:"m",body:[{e:"wrap",child:{e:"num",n:3}}]}]});}}`, "union.declarations")
}

func TestGogenA9AliasKinds(t *testing.T) {
	for name, source := range map[string]string{
		"primitive":  `type Value=string;function read(v:Value):string{return v;}class Probe {static run():string{return read("ok");}}`,
		"union":      `type Value=string|number;function read(v:Value):string{return typeof v+":"+v;}class Probe {static run():string{return read("s")+";"+read(2);}}`,
		"record":     `type Value<T>={readonly [key:string]:T|undefined};function read(v:Value<string>):string{return Object.keys(v).join(",")+":"+JSON.stringify(v);}class Probe {static run():string{return read({z:undefined,a:"ok","2":"two"});}}`,
		"constraint": `function read<Value extends string|number>(v:Value):string{return typeof v+":"+v;}class Probe {static run():string{return read("ok")+";"+read(3);}}`,
	} {
		t.Run(name, func(t *testing.T) {
			refusal := ""
			if name == "record" {
				refusal = "own-property order/toJSON"
			}
			gogenTSOracle(t, source, refusal)
		})
	}
}

func TestGogenA9MappedRecursiveGraph(t *testing.T) {
	gogenTSOracle(t, `type Raw={e:"field";base:Raw}|{e:"var";name:string};type RuntimeValue<T>=T extends Raw ? Expr : T extends readonly (infer V)[] ? readonly RuntimeValue<V>[] : T extends object ? Fields<T>:T;type Fields<T>={[K in keyof T]:RuntimeValue<T[K]>};type Expr=Fields<Raw>;type Cache<T>=T extends readonly (infer V)[] ? readonly Cache<V>[] : T extends object ? {[K in keyof T]:Cache<T[K]>}:T;interface Class {name:string;body:readonly Expr[];}type ClassIR=Cache<Class>;function read(c:ClassIR):string{return c.name+":"+JSON.stringify(c);}class Probe {static run():string{return read({name:"C",body:[{e:"field",base:{e:"var",name:"x"}}]});}}`, "union.declarations")
}

func TestGogenA9InferredEmptyArray(t *testing.T) {
	gogenTSOracle(t, `class Probe {static run():string {const value={name:"n",items:[],extra:undefined};return Object.keys(value).join(",")+":"+JSON.stringify(value);}}`, "")
}

func TestGogenA9StructuralOrderAndIdentity(t *testing.T) {
	gogenTSOracle(t, `function first(x:{readonly z:string;a:number}):string{return Object.keys(x).join(",")+":"+JSON.stringify(x);}function second(x:{a:number;z:string;extra?:boolean}):string{return first(x);}
 class Probe {static trace="";static mark(k:string):string {Probe.trace+=k;return k;}static run():string {const a={z:Probe.mark("z"),a:2};const b={a:1,z:Probe.mark("a")};return second(a)+";"+second(b)+";"+(a===a)+":"+Probe.trace;}}`, "")
}

func TestGogenA9MappedIntersection(t *testing.T) {
	gogenTSOracle(t, `type Raw={e:"leaf";name:string}|{e:"call";args:readonly Raw[]};type Cache<T>=T extends {e:"call"} ? Omit<T,"args"> & {readonly args?:readonly Cache<Raw>[]} : T extends readonly (infer V)[] ? readonly Cache<V>[] : T extends object ? {[K in keyof T]:Cache<T[K]>}:T;interface Class {name:string;body:readonly Raw[];}type ClassIR=Cache<Class>;function read(c:ClassIR):string {return c.name+":"+JSON.stringify(c);}class Probe {static run():string{return read({name:"C",body:[{e:"call",args:[{e:"leaf",name:"x"}]}]});}}`, "union.declarations")
}

func TestGogenA9CrossFileMappedShape(t *testing.T) {
	types := `export type Raw={e:"leaf";name:string}|{e:"call";args:readonly Raw[]};`
	cache := `import type {Raw} from "./types";type Cache<T>=T extends {e:"call"} ? Omit<T,"args"> & {readonly args?:readonly Cache<Raw>[]} : T extends readonly (infer V)[] ? readonly Cache<V>[] : T extends object ? {[K in keyof T]:Cache<T[K]>}:T;interface Class {name:string;ctorParams?:readonly {name:string}[];body:readonly Raw[];}export type ClassIR=Cache<Class>;`
	probe := `import type {ClassIR} from "./cache";function read(c:ClassIR):string {return c.name+":"+Object.keys(c).join(",")+":"+JSON.stringify(c);}class Probe {static run():string{return read({name:"C",body:[{e:"call",args:[{e:"leaf",name:"x"}]}]});}}`
	nodeSource := strings.ReplaceAll(types+strings.ReplaceAll(cache, `import type {Raw} from "./types";`, "")+strings.ReplaceAll(probe, `import type {ClassIR} from "./cache";`, ""), "export ", "")
	want := conversionOracle(t, nodeSource)
	p := lowerStatementsProbe(t, map[string]string{"types.ts": types, "cache.ts": cache, "probe.ts": probe}, []string{"types.ts", "cache.ts", "probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	_, err := emitABAP(t, p)
	if err == nil || !strings.Contains(err.Error(), "union.") || !strings.Contains(err.Error(), "types.ts:") {
		t.Fatalf("expected located ABAP union refusal, got %v", err)
	}
}

func TestGogenA9InferredCallable(t *testing.T) {
	gogenTSOracle(t, `class Probe {static run():string {const directory=true;const entry={name:"n",isDirectory:()=>directory};return entry.name+":"+entry.isDirectory()+":"+Object.keys(entry).join(",")+":"+JSON.stringify(entry)+":"+JSON.stringify([entry.isDirectory]);}}`, "own-property order/toJSON")
}

func TestGogenA9NullAndUndefined(t *testing.T) {
	gogenTSOracle(t, `class Probe {static run():string {const value={name:"n",value:null,optional:undefined};return Object.keys(value).join(",")+":"+("optional" in value)+":"+typeof value.value+":"+JSON.stringify(value);}}`, "")
}
