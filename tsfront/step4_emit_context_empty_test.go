package tsfront

import "testing"

func TestStep4EmitEmptyContextViewNodeOracle(t *testing.T) {
	a20Oracle(t, `type Expr={readonly e:"str";readonly value:string}|{readonly e:"wrap";readonly x:Expr;readonly metadata?:{}};
 interface Arg{readonly name:string;readonly value?:Expr}
 interface Method{readonly name?:string;readonly params:readonly Arg[];readonly body?:readonly Expr[]}
 interface Cls{readonly name:string;readonly methods:readonly Method[];readonly constructor?:Method}
 interface Context{readonly cls:Partial<Omit<Cls,"constructor">>;method?:Partial<Method>;loop?:number}
 type Render=(e:Expr,ctx:Context)=>string;
 function render(e:Expr,ctx:Context):string{ctx.loop=(ctx.loop??0)+1;return e.e+ctx.loop+(ctx.cls.name??"?")+(ctx.method?.name??"-");}
 function local(e:Expr,expr:Render):string{const ctx={cls:{},method:{}};return expr(e,ctx)+":"+expr(e,ctx);}
 class Probe{static run():string{return local({e:"str",value:"a"},render);}}`)
}
