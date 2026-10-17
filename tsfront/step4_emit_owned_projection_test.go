package tsfront

import "testing"

// ownedStatement reads storage metadata through a checked payload assertion.
func TestStep4EmitOwnedProjectionNodeOracle(t *testing.T) {
	a20Oracle(t, `type Ty={readonly k:"x";readonly len:number}|{readonly k:"xs"};
 type Expr={readonly e:"var";readonly name:string;readonly type:Ty}|{readonly e:"wrap";readonly x:Expr;readonly type:Ty}|{readonly e:"int";readonly value:number};
 type OwnedPlace=Extract<Expr,{e:"var"}>;
 type Stmt={readonly s:"assign";readonly target:Expr;readonly value:Expr}|{readonly s:"clear";readonly target:Expr};
 function has(e:Expr):boolean{return e.e==="var";}
 function emit(st:Stmt):string{if(st.s==="assign"&&has(st.target)&&(st.target as OwnedPlace).type.k==="x")return (st.target as {readonly type:Extract<Ty,{k:"x"}>}).type.len+":"+(st.value.e==="int"?st.value.value:0);return "-";}
 class Probe{static run():string{return emit({s:"assign",target:{e:"var",name:"v",type:{k:"x",len:3}},value:{e:"int",value:7}})+":"+emit({s:"clear",target:{e:"var",name:"w",type:{k:"xs"}}});}}`)
}
