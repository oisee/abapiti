package tsfront

import "testing"

func TestStep4EmitOptionalStructuralViewNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Base {readonly name?:string} interface Full {name:string;extra:string} interface Slot {readonly value?:Base}
 class Probe {static calls=0; static source(x:Full|undefined):Full|undefined{Probe.calls++;return x;}
 static widen(x:Full|undefined):Base|undefined{return Probe.source(x);}
 static show(x:Base):string{return x.name??"none";}
 static run():string{const a:Full={name:"a",extra:"keep"};const slot:Slot={value:a};const b:Base=a;const present=Probe.widen(a),absent=Probe.widen(undefined);a.name="live";return Probe.show(a)+":"+(b===a)+":"+(slot.value===a)+":"+(present===a)+":"+present?.name+":"+(absent===undefined)+":"+a.extra+":"+Probe.calls;}}`, "")
}
