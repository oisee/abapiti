package tsfront

import (
	"fmt"
	"testing"
)

// An object-typed union field accepts every JS object, arrays included: a
// parsed array stored as an opaque token must pass the family validator, the
// same domain dynObject bridges on reads. Narrower ref fields keep their check.
func TestObjectRootUnionFieldAcceptsArrayNodeOracle(t *testing.T) {
	source := `type LoopToken = object;
 type Stmt = {k:"loop"; token:LoopToken; body:Stmt[]} | {k:"delete_current"; token:LoopToken};
 type LoopArm = {k:"loop"; token:LoopToken; body:Stmt[]};
 class Walker {
 names:Map<LoopToken,string> = new Map();
 last:LoopToken|undefined = undefined;
 repeat:number = 0;
 walk(st:Stmt):void {
 if (st.k==="loop") {this.names.set(st.token,"loop");for (const inner of st.body) this.walk(inner);return;}
 const token = st.token;
 if (this.last===token) this.repeat++;
 this.last = token;
 }
 }
 class Probe {
 static mk(token:LoopToken, body:Stmt[]):Stmt {const base = {k:"loop" as const, token, body};const arm:LoopArm = {...base};return arm;}
 static run():string {
 const arr: unknown[] = JSON.parse('[1]');
 const asObj: object = JSON.parse('[2]');
 const st = Probe.mk(arr,[{k:"delete_current", token:arr},{k:"delete_current", token:arr}]);
 const st2 = Probe.mk(asObj,[{k:"delete_current", token:asObj}]);
 const w = new Walker();
 w.walk(st);
 w.walk(st2);
 let tok: object = {};
 if (st.k==="loop") tok = st.token;
 const m = new Map<object,number>();
 m.set(tok,1); m.set(arr,2);
 return w.names.get(arr)+":"+w.names.size+":"+w.repeat+":"+(tok===arr)+":"+m.size+":"+w.names.get(asObj)+":"+(w.last===asObj);
 }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
