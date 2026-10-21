package tsfront

import "testing"

// A property name in an object literal ({path: ...}) is not a read of the
// enclosing local path: lifting scan from scanBody must not capture it
// (sink-locals.ts scan/scanBody).
func TestStep4EmitLiftedPropertyNameNotCapturedNodeOracle(t *testing.T) {
	a20Oracle(t, `type V=readonly V[]|string;
 interface Use{readonly path:(readonly V[])[];readonly n:number}
 function sink(body:readonly V[]):string{const uses=new Map<string,Use>();
  function scan(node:V,path:readonly (readonly V[])[]):void{if(typeof node==="string"){if(!uses.has(node))uses.set(node,{path:[...path],n:path.length});return;}scanBody(node,[...path,node]);}
  function scanBody(b:readonly V[],path:readonly (readonly V[])[]):void{for(const s of b)scan(s,path);}
  scanBody(body,[body]);return [...uses].map(([k,u])=>k+u.n+":"+u.path.length).join(",");}
 class Probe {static run():string{return sink(["a",["b","a",["x"]]]);}}`)
}
