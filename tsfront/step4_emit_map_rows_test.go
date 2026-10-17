package tsfront

import "testing"

// new Map(rows) coerces each row field inside the loop: a widening temporary
// reads the row variable (sink-locals.ts scan, Use.first).
func TestStep4EmitMapRowsCoerceInLoopNodeOracle(t *testing.T) {
	a20Oracle(t, `type S={readonly s:"a";readonly n:number}|{readonly s:"b";readonly body:readonly S[]};
 type I<T>=T extends readonly (infer V)[]?readonly I<V>[]:T extends object?{readonly [K in keyof T]?:I<T[K]>}&Pick<T,Extract<keyof T,"s">>:T;
 type St=I<S>;
 interface Use{readonly first:Map<string,St|undefined>}
 function f(names:readonly string[],top:ReadonlyMap<string,St>):string{const uses=new Map<string,Use>();
  if(!uses.has("k"))uses.set("k",{first:new Map(names.map((b):[string,St|undefined]=>[b,top.get(b)]))});
  let out="";for(const [k,v] of uses.get("k")!.first){out+=k+(v===undefined?"-":v.s);}return out;}
 class Probe {static run():string{const top=new Map<string,St>();top.set("x",{s:"a",n:5});top.set("y",{s:"b",body:[]});return f(["x","z","y"],top);}}`)
}
