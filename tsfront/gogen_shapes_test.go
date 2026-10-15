package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestGogenResolvedDataShapes(t *testing.T) {
	for name, source := range map[string]string{
		"omit":               `interface Attribute {name:string;value:number;} interface Supported extends Omit<Attribute,"value"> {readonly value?:string;} class Probe {static run():string {const a:Supported={name:"n",value:"v"};return a.name+":"+a.value;}}`,
		"utilities":          `interface Attribute {name:string;value:number;} type Selected=Readonly<Pick<Attribute,"name">>; interface Supported extends Selected,Partial<Pick<Attribute,"value">> {tag:string;} class Probe {static run():string {const a:Supported={name:"n",tag:"v"};return a.name+":"+a.tag;}}`,
		"inherited":          `interface Context {name:string;} interface ByteContext extends Context {value:string;} class Probe {static run():string {const a:ByteContext={name:"n",value:"v"};return a.name+":"+a.value;}}`,
		"narrowed":           `interface Context {name:string;value?:string;} interface ByteContext extends Context {value:string;} function read(c:Context):string{return c.name+":"+c.value;} class Probe {static run():string {const a:ByteContext={name:"n",value:"v"};return read(a);}}`,
		"inherited_only":     `interface Attribute {name:string;value:string;} interface Supported extends Readonly<Attribute> {} class Probe {static run():string {const a:Supported={name:"n",value:"v"};return a.name+":"+a.value;}}`,
		"narrowed_reference": `interface Item {text:string;} interface Context {item?:Item;} interface ByteContext extends Context {item:Item;loop:number;} function read(c:Context):string{return c.item ? c.item.text : "absent";} class Probe {static run():string {const a:ByteContext={item:{text:"n:v"},loop:2};return read(a);}}`,
		"alias_base":         `type Context={name:string;}; interface ByteContext extends Context {value:string;} class Probe {static run():string {const a:ByteContext={name:"n",value:"v"};return a.name+":"+a.value;}}`,
	} {
		t.Run(name, func(t *testing.T) { gogenOracle(t, source, "n:v") })
	}
}

func TestGogenDataMethodShape(t *testing.T) {
	gogenOracle(t, `interface Renderers {tag:string;value(x:number):string;} class Probe {static run():string {const r:Renderers={tag:"v",value:(x:number):string=>"n:"+x};return r.value(3)+r.tag;}}`, "n:3v")
}

func TestGogenParenthesizedCallee(t *testing.T) {
	gogenOracle(t, `function apply(fn:(n:number)=>number):number {return (fn)(2)+((fn))(3);} class Probe {static run():string {return ""+apply((n:number):number=>n*10);}}`, "50")
}

func TestGogenJSDocCallee(t *testing.T) {
	runMjsOracle(t, `/** @param {(n:number)=>number} fn */ function apply(fn) {return (/** @type {(n:number)=>number} */ (fn))(2);} export class Probe {static run() {return apply(n=>n*10);}}`, true)
}

func TestGogenNarrowedShapeIdentity(t *testing.T) {
	source := `interface Context {value?:string;} interface ByteContext extends Context {value:string;} class Probe {static run():string {const a:ByteContext={value:"v"};return a.value;}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	var base, derived *hir.Class
	for _, c := range p.Classes {
		if c.Name == "probe.ts.Context" {
			base = c
		}
		if c.Name == "probe.ts.ByteContext" {
			derived = c
		}
	}
	if base == nil || derived == nil || base == derived || len(base.Fields) != 1 || len(derived.Fields) != 1 || base.Fields[0].Type.Kind != hir.Optional || derived.Fields[0].Type.Kind != hir.String {
		t.Fatalf("narrowed interface must retain a distinct required field: base=%+v derived=%+v", base, derived)
	}
	gogenCheck(t, p, "probe.ts", source, "v")
}

func TestGogenDataMethodMemberRefusal(t *testing.T) {
	source := `interface Renderers {tag:string;value(x:number):string;value(x:string):string;}`
	_, ds := lowerProbe(t, sourceProbe(t, source))
	for _, d := range ds {
		if d.Category == "skipped-interface-member" && strings.Contains(d.Message, "member value has unlowered types") && strings.Contains(d.Loc, "input.ts:1:") {
			return
		}
	}
	t.Fatalf("expected located refusal naming overloaded member value: %v", ds)
}

func TestGogenInheritedShapeLocations(t *testing.T) {
	base := `interface Context {name:string;}`
	source := `type Padding="` + strings.Repeat("x", 1000) + `";interface ByteContext extends Context {value:string;} class Probe {static run():string {const a:ByteContext={name:"n",value:"v"};return a.name+":"+a.value;}}`
	p := lowerStatementsProbe(t, map[string]string{"base.ts": base, "probe.ts": source}, []string{"base.ts", "probe.ts"})
	gogenCheck(t, p, "probe.ts", base+source, "n:v")
}

// An interface extending a class lists the class's methods among its
// properties; calls of those methods on class instances stay method calls
// (they read this), never stored-function calls.
func TestGogenInterfaceExtendsClassMethods(t *testing.T) {
	gogenOracle(t, `class Ident {private name:string; constructor(name:string){this.name=name;} getName():string {return this.name;} equals(o:Ident):boolean {return o.getName()===this.getName();}}
interface Def extends Ident {kind:string;}
class Probe {static run():string {const a=new Ident("n");const b=new Ident("n");return (a.equals(b)?"n":"x")+":"+a.getName().replace("n","v");}}`, "n:v")
}

// A data shape's method signature holds a stored function: taking it as a
// value (h.rowValue passed as a callback) reads that function.
func TestGogenStoredFunctionValue(t *testing.T) {
	gogenOracle(t, `interface Renderers {tag:string;value(x:number):string;}
function apply(f:(x:number)=>string):string {return f(3);}
class Probe {static run():string {const r:Renderers={tag:"v",value:(x:number):string=>"n:"+x};return apply(r.value).replace("3","")+r.tag;}}`, "n:v")
}
