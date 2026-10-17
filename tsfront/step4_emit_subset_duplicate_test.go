package tsfront

import "testing"

// A runtime copy and its raw source member share one storage layout; a
// union holding both narrows to one alternative, not two with the same id.
func TestStep4EmitSubsetSharedLayoutNodeOracle(t *testing.T) {
	a20Oracle(t, `type Ty={readonly k:"i"}|{readonly k:"t";readonly row:Ty};
 interface Meta{readonly conv?:boolean}
 type Raw=Meta&({readonly e:"var";readonly name:string;readonly type:Ty}|{readonly e:"field";readonly base:Raw;readonly name:string;readonly type:Ty});
 type RuntimeValue<T>=T extends Ty?T:T extends Raw?Expr:T;
 type RuntimeFields<T>=T extends object?{readonly [K in keyof T]:RuntimeValue<T[K]>}:T;
 type Expr=RuntimeFields<Raw>;
 class Probe {static name(x:Expr|Raw):string{if(x.e==="var")return x.name+(x.conv?"!":"");return "f:"+x.name+":"+Probe.name(x.base);}
 static run():string{const v:Raw={e:"var",name:"a",type:{k:"i"}};const e:Expr={e:"field",base:{e:"var",name:"b",type:{k:"i"},conv:true},name:"c",type:{k:"i"}};return Probe.name(v)+":"+Probe.name(e);}}`)
}

// Shared-layout members with different secondary literal domains are not
// joined field by field: that join would admit mixed combinations.
func TestStep4EmitSubsetSharedLayoutSecondaryNodeOracle(t *testing.T) {
	a20Oracle(t, `type Ty={readonly k:"i"}|{readonly k:"t";readonly row:Ty};
 type Raw={readonly e:"var";readonly m:"a"|"b";readonly n:"x"|"y";readonly type:Ty}|{readonly e:"field";readonly base:Raw;readonly m:"a"|"b";readonly n:"x"|"y";readonly type:Ty};
 type RuntimeValue<T>=T extends Ty?T:T extends Raw?Expr:T;
 type RuntimeFields<T>=T extends object?{readonly [K in keyof T]:RuntimeValue<T[K]>}:T;
 type Expr=RuntimeFields<Raw>;
 type Mixed=Extract<Expr,{e:"var"}>&{readonly m:"a";readonly n:"x"}|Extract<Raw,{e:"var"}>&{readonly m:"b";readonly n:"y"};
 class Probe {static show(x:Mixed):string{return x.e+x.m+x.n;}
 static run():string{return Probe.show({e:"var",m:"a",n:"x",type:{k:"i"}})+":"+Probe.show({e:"var",m:"b",n:"y",type:{k:"i"}});}}`)
}
