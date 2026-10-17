package tsfront

import "testing"

func TestStep4EmitNamedFamilyRefinementNodeOracle(t *testing.T) {
	a20Oracle(t, `type Type={readonly k:"str"}|{readonly k:"packed";readonly dec:number};
 type Expr={readonly e:"num";readonly value:number}|{readonly e:"conv";readonly kind:string;readonly to:Type;readonly x:Expr};
 interface Packed extends Extract<Expr,{e:"conv"}>{readonly kind:"packed";readonly to:Extract<Type,{k:"packed"}>}
 function isPacked(e:Expr):e is Packed{return e.e==="conv"&&e.kind==="packed";}
 class Probe {static show(e:Expr):string{if(!isPacked(e))return "other";return e.to.dec+":"+e.x.e;}
 static run():string{const e:Expr={e:"conv",kind:"packed",to:{k:"packed",dec:2},x:{e:"num",value:1}};return Probe.show(e)+":"+Probe.show({e:"num",value:7});}}`)
}
