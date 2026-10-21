package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStep4ErasedArrayContextNodeOracle(t *testing.T) {
	source := `interface Entry {key:string[];columns:{name:string;key:boolean}[];}
 class Probe {
 static values:any[]=["a"];
 static read(view:boolean,fail:boolean):Entry {
 let key:any[]=[];
 try {key=view?[]:Probe.values.map(x=>x);if(fail)throw new Error("reset");}catch {key=[];}
 const columns=["a"].map(name=>({name,key:key.includes(name)}));
 const entry:Entry={key,columns};return entry;
 }
 static typed(values:any[]):string[]{return values;}
 static erased(values:string[]):any[]{return values;}
 static run():string {
 const e=Probe.read(false,false);const typed=Probe.typed(Probe.values);const erased=Probe.erased(typed);
 typed.push("b");erased[0]="changed";Probe.values.push("c");
 return e.key.join(",")+":"+e.columns[0].key+":"+Probe.read(true,false).key.length+":"+Probe.read(false,true).key.length+":"+(typed===Probe.values)+":"+(erased===Probe.values)+":"+typed.join(",");
 }
 }`
	for _, trap := range []bool{false, true} {
		t.Run(fmt.Sprintf("trap=%t", trap), func(t *testing.T) {
			p, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: trap})
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(p); len(errs) != 0 {
				t.Fatal(errs)
			}
			conditionalMatchesNode(t, source, p, "Probe")
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
		})
	}
}

func TestStep4ErasedOptionalArrayAndTupleNodeOracle(t *testing.T) {
	source := `class Probe {
 static calls=0;
 static optional(value:any[]|undefined):string[]|undefined {Probe.calls++;return value;}
 static erased(value:string[]|undefined):any[]|undefined {return value;}
 static tuple():[string,string] {return ["a","b"];}
 static run():string {
 const a=Probe.tuple();const view=Probe.optional(Probe.erased(a));
 if(view===undefined)return "bad";
 view[0]="x";const chosen=Probe.calls>0?(Probe.erased(view)??[]):a;chosen.push("c");
 const mixed:[string,number]=["tag",7];
 return (view===a)+":"+(chosen===a)+":"+(Probe.erased(view)===a)+":"+a.join(",")+":"+(Probe.optional(undefined)===undefined)+":"+(Probe.erased(undefined)===undefined)+":"+Probe.calls+":"+mixed[0]+":"+mixed[1];
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestStep4ErasedArrayInvalidElementRefusal(t *testing.T) {
	source := `class Probe {
 static typed(values:any[]):string[]{return values;}
 static read(values:any[]):string {return Probe.typed(values).join(",");}
 static invalid():string {return Probe.read(["valid",7]);}
 static later():string {const values:any[]=["valid"];const typed=Probe.typed(values);values[0]=7;return typed.join(",");}
 static write():string {const strings:string[]=["valid"];const values:any[]=strings;values[0]=7;return strings.join(",");}
 static run():string {return Probe.read(["valid"]);}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf(`for _,f:=range []func()jsString{%s,%s,%s}{func(){defer func(){switch x:=recover().(type){case trap,rangeFault:default:panic(fmt.Sprintf("missing checked array refusal: %%T %%v",x,x))}}();f()}()};fmt.Println("refused")`, goEntry("probe.ts.Probe", "invalid"), goEntry("probe.ts.Probe", "later"), goEntry("probe.ts.Probe", "write")))
	if got != "refused\n" {
		t.Fatal(got)
	}
	for _, method := range []string{"invalid", "later", "write"} {
		storageABAPProbe(t, p, "probe.ts.Probe", "valid", method, "cx_sy_move_cast_error")
	}
}

func TestStep4EvolvingJSDocArrayCallback(t *testing.T) {
	runMjsOracleTypes(t, `export interface Entry {key:string[];columns:{name:string,key:boolean}[];}
 export class Table {listKeys():any[]{return ["a"];}}`,
		`import {Table} from "./types.ts";
 /** @param {unknown} s @returns {string} */ const upper=s=>String(s).toUpperCase();
 /** @param {()=>void} callback */ function artifact(callback){callback();}
 export class Probe {
 /** @param {Table|undefined} table @param {boolean} view @param {boolean} fail @returns {import("./types.ts").Entry} */
 static read(table,view,fail) {
 /** @type {import("./types.ts").Entry[]} */ const out=[];
 artifact(()=>{let key=[];
 try {key=view?[]:(table?.listKeys?.()??[]).map(upper);if(fail)throw new Error("reset");}catch {key=[];}
 /** @type {import("./types.ts").Entry["columns"]} */ const columns=["a"].map(name=>({name,key:key.includes(upper(name))}));
 /** @type {import("./types.ts").Entry} */ const entry={key,columns};out.push(entry);
 });return out[0];
 }
 /** @returns {string} */ static run(){return Probe.read(new Table(),false,false).key.join(",")+":"+Probe.read(undefined,false,false).key.length+":"+Probe.read(new Table(),false,true).key.length;}
 }`, true, true)
}

func TestStep4ArrayFallbackEffectsNodeOracle(t *testing.T) {
	source := `class Holder {
 values:string[]=["x"];calls=0;
 read(flag:boolean):string[]|null|undefined {this.calls++;return flag?this.values:this.calls>1?null:undefined;}
 }
 class Probe {
 static inputs=0;
 static subject(value:string):string {Probe.inputs++;return value;}
 static optional(fn:(()=>string[]|null|undefined)|undefined):string[] {return fn?.()||[];}
 static value(value:false|number|string|null|undefined):string {const picked=value||"fallback";return String(picked);}
 static run():string {
 const h=new Holder();const a=h.read(false)??[];const b=h.read(false)||[];const c=h.read(true)??[];const d=h.read(true)||[];
 const matched=Probe.subject("foo").match(/^(foo)$/)??[];const missed=Probe.subject("miss").match(/^(foo)$/)||[];
 c.push("late");
 return a.length+":"+b.length+":"+(c===h.values)+":"+(d===h.values)+":"+d.join(",")+":"+h.calls+":"+matched.join(",")+":"+missed.length+":"+Probe.inputs+":"+Probe.optional(undefined).length+":"+Probe.value(false)+":"+Probe.value(0)+":"+Probe.value(3)+":"+Probe.value("")+":"+Probe.value("yes");
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestStep4BoxedArrayStorageNodeOracle(t *testing.T) {
	source := `class Probe {
 static run():string {
 const strings:string[]=["a"];const boxed:any=strings;const erased:any[]=boxed;
 const typed:string[]=erased;typed.push("b");erased[0]="x";
 return (erased===strings)+":"+(typed===strings)+":"+strings.join(",");
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestStep4ErasedArrayInvalidPayloadRefusal(t *testing.T) {
	source := `class Probe {
 static typed(values:any[]):string[]{return values;}
 static absent():string {const value:any=undefined;const values:any[]=value;return Probe.typed(values).join(",");}
 static object():string {const value:any={};const values:any[]=value;return Probe.typed(values).join(",");}
 static nullValue():string {const value:any=null;const values:any[]=value;return Probe.typed(values).join(",");}
 static nonIndex():string {const native:string[]=["ok"];const value:any=native;return String(delete value.foo);}
 static run():string {return "valid";}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf(`for _,f:=range []func()jsString{%s,%s,%s,%s}{func(){defer func(){switch x:=recover().(type){case trap,rangeFault:default:panic(fmt.Sprintf("missing checked array refusal: %%T %%v",x,x))}}();f()}()};fmt.Println("refused")`, goEntry("probe.ts.Probe", "absent"), goEntry("probe.ts.Probe", "object"), goEntry("probe.ts.Probe", "nullValue"), goEntry("probe.ts.Probe", "nonIndex")))
	if got != "refused\n" {
		t.Fatal(got)
	}
	for _, method := range []string{"absent", "object", "nullValue", "nonIndex"} {
		storageABAPProbe(t, p, "probe.ts.Probe", "valid", method)
	}
}
