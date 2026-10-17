package tsfront

import "testing"

func TestStep4EmitArrayShapeViewNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Param {readonly name:string} interface FullParam extends Param {readonly defaultOwner?:string}
 class Probe {static view(xs:readonly FullParam[]):readonly FullParam[]{return xs;}
 static read(xs:readonly FullParam[]):string{return xs.map(p=>p.name+":"+(p.defaultOwner??"none")).join(",");}
 static run():string{const p:Param={name:"x"};const xs:Param[]=[p];const alias=Probe.view(xs);xs.push({name:"y"});return Probe.read(alias)+":"+(alias===xs)+":"+(alias[0]===p)+":"+xs.length;}}`, "")
}
