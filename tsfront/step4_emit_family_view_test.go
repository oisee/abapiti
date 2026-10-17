package tsfront

import "testing"

func TestStep4EmitFamilyProbeViewNodeOracle(t *testing.T) {
	a20Oracle(t, `type Type={readonly k:"str"}|{readonly k:"table";readonly row:Type};type TypeProbe=Type&{readonly name?:string;readonly row?:TypeProbe};
 class Probe {static show(t:TypeProbe):string{return t.k==="table"?"["+Probe.show(t.row)+"]":(t.name??"s");}
 static widen(t:Type):TypeProbe{return t;}static optional(t:Type|undefined):TypeProbe|undefined{return t;}
 static run():string{const t:Type={k:"table",row:{k:"table",row:{k:"str"}}};const a=Probe.widen(t),b=Probe.optional(t);return Probe.show(t)+":"+Probe.show(a)+":"+(a===t)+":"+(b===t)+":"+(Probe.optional(undefined)===undefined);}}`)
}
