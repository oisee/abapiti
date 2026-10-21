package tsfront

import "testing"

// A literal context passed where a property-only interface with an optional
// nested record slot is expected keeps one live object
// (emit-local-constants.ts localConstant: ctx = {cls: {}, method: {}}).
func TestStep4EmitNestedOptionalShapeViewNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Arg{readonly name?:string;readonly dir?:string}
 function argName(a:Arg):string{return a.name??"none";}
 interface Meth{readonly name:string;readonly k:number}
 interface Context{readonly cls:Partial<Meth>;loop?:number;method?:Partial<Meth>}
 type Render=(e:string,ctx:Context)=>string;
 function render(e:string,ctx:Context):string{ctx.loop=(ctx.loop??0)+1;const r=e+ctx.loop+(ctx.method?.name??"?")+(ctx.method?.k??"-");return r;}
 function local(xs:readonly string[],expr:Render):string{const ctx={cls:{},method:{}};return xs.map((x):string=>expr(x,ctx)).join(",")+":"+argName({});}
 class Probe {static run():string{return local(["a","b","c"],render);}}`)
}
