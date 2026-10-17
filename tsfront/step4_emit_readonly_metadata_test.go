package tsfront

import "testing"

func TestStep4EmitReadonlyInterfaceFieldNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface A {readonly name:string} interface B {readonly name:string}
 interface Left {readonly rows:readonly A[]} interface Right {readonly rows:readonly B[]}
 class Probe {static read(x:Left):string{return x.rows.map(i=>i.name).join(",");}
 static run():string{const row:B={name:"b"};const right:Right={rows:[row]};const left:Left=right;return Probe.read(right)+":"+(left===right)+":"+(left.rows===right.rows)+":"+(left.rows[0]===row);}}`, "")
}
