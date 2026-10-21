package tsfront

import "testing"

// The asserted callback returns a narrower recursive expression graph. Its
// declared implementation result must not leak through the direct call.
func TestStep4EmitAssertedFactoryResultNodeOracle(t *testing.T) {
	a20Oracle(t, `type Ty={readonly k:"i"}|{readonly k:"struct";readonly name:string};
 interface Prime{readonly raw:Raw;readonly expr:Expr}
 interface Arg{readonly value?:Raw}
 interface RuntimeArg{readonly value?:Expr}
 type Raw={readonly e:"call";readonly args:readonly Arg[]}|{readonly e:"struct";readonly type:Ty;readonly x:Raw};
 type Expr={readonly e:"call";readonly args:readonly RuntimeArg[]}|{readonly e:"struct";readonly type:Extract<Ty,{k:"struct"}>;readonly x:Expr};
 type Factory=()=>Extract<Expr,{e:"call"}>;
 function omitted():Raw{return {e:"call",args:[{value:{e:"struct",type:{k:"struct",name:"S"},x:{e:"call",args:[]}}}]};}
 function factory(yes:boolean):readonly RuntimeArg[]{const args=yes?(omitted as Factory)().args:[];return args;}
 class Probe{static run():string{return factory(true).map(a=>a.value?.e??"-").join(",")+":"+factory(false).length;}}`)
}
