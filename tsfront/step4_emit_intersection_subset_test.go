package tsfront

import "testing"

func TestStep4EmitIntersectionSubsetNodeOracle(t *testing.T) {
	a20Oracle(t, `type Ty={readonly k:'i'}|{readonly k:'table';readonly row:Ty};
 type Raw={readonly e:'int'|'float';readonly value:number;readonly type?:Ty}|{readonly e:'link';readonly x:Raw;readonly type:Ty}|{readonly e:'struct';readonly x:Raw;readonly type:Ty};
 type RuntimeValue<T>=T extends Raw?Expr:T extends readonly (infer V)[]?readonly RuntimeValue<V>[]:T extends object?RuntimeFields<T>:T;
 type RuntimeFields<T>=T extends object?{readonly[K in keyof T]:RuntimeValue<T[K]>}:T;
 type Expr=RuntimeFields<Exclude<Raw,{e:'struct'}>>|(RuntimeFields<Omit<Extract<Raw,{e:'struct'}>,'type'>>&{readonly type:Extract<Ty,{k:'table'}>});
 type TypedExpr=Exclude<Expr,{e:'int'|'float'}>|(Extract<Expr,{e:'int'|'float'}>&{readonly type:Ty});
 type Split=Expr&({readonly e:'int'}|{readonly e:'float'}|{readonly e:'link'}|{readonly e:'struct'});
 class Probe {static base(e:Expr):string{return e.e;} static show(e:Split):string{return e.e;}
 static typed(e:TypedExpr):string{return e.e;}
 static run():string {const i:Expr={e:'int',value:2};const f:Expr={e:'float',value:3};const s:Expr={e:'struct',x:i,type:{k:'table',row:{k:'i'}}};return Probe.show(i as Split)+':'+Probe.show(f as Split)+':'+Probe.show(s)+':'+Probe.typed(s);}}`)
}
