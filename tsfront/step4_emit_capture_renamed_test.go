package tsfront

import "testing"

// A for-of binding that shadows an earlier inlined callback parameter is
// renamed; a closure capturing it must read the renamed local
// (emit-table-move.ts analyzeTableMoves: flatMap((cls) => ...) then
// for (const [cls, method] of bodies) with a walker callback).
func TestStep4EmitClosureCaptureOfRenamedBindingNodeOracle(t *testing.T) {
	a20Oracle(t, `interface C{readonly name:string;readonly ms:readonly string[]}
 function walk(xs:readonly string[],visit:(x:string)=>void):void{for(const x of xs)visit(x);}
 function run(classes:readonly C[]):string{const bodies:[C,string][]=classes.flatMap((cls):[C,string][]=>cls.ms.map((m):[C,string]=>[cls,m]));let out="";
  for(const [cls,method] of bodies){walk([method,"z"],(x):void=>{out+=cls.name+"."+x+";";});}return out;}
 class Probe {static run():string{return run([{name:"A",ms:["f","g"]},{name:"B",ms:["h"]}]);}}`)
}
