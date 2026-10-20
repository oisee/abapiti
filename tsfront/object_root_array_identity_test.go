package tsfront

import (
	"fmt"
	"testing"
)

// An array stored in an object-typed field and read back through a view or
// carrier must be the same reference as the array obtained any other way:
// typed array locals, any[] fields and dynamic re-reads all compare === and
// are one Map/Set key; writing a read back into the field keeps one box.
func TestObjectRootArrayIdentityNodeOracle(t *testing.T) {
	source := `type LoopToken = object;
 type Stmt = {k:"loop"; token:LoopToken; body:Stmt[]} | {k:"delete_current"; token:LoopToken};
 type LoopArm = {k:"loop"; token:LoopToken; body:Stmt[]};
 interface Holder { o: object; arr: unknown[]; }
 class Walker {
 last:LoopToken|undefined = undefined;
 repeat:number = 0;
 walk(st:Stmt):void {
 if (st.k==="loop") {for (const inner of st.body) this.walk(inner);return;}
 const token = st.token;
 if (this.last===token) this.repeat++;
 this.last = token;
 }
 }
 class Probe {
 static mk(token:LoopToken, body:Stmt[]):Stmt {const base = {k:"loop" as const, token, body};const arm:LoopArm = {...base};return arm;}
 static run():string {
 const a: unknown[] = [];
 const holder = {o: a as object};
 const h:Holder = JSON.parse('{"o":{},"arr":[]}');
 h.o = a;
 h.arr = a;
 const first: object = h.o;
 h.o = first;
 const second: object = h.o;
 const st = Probe.mk(a,[{k:"delete_current", token:a},{k:"delete_current", token:a},{k:"delete_current", token:{}}]);
 const w = new Walker();
 w.walk(st);
 let viaFamily = false;
 if (st.k==="loop") viaFamily = st.token===a;
 const p:Holder = JSON.parse('{"o":{},"arr":[1]}');
 p.o = p.arr;
 const viaView:object = p.o;
 const typed:unknown[] = p.arr;
 const field = {o: p.arr as object};
 const parsed = p.arr;
 const seen = new Map<object,string>();
 seen.set(viaView,"v");
 const ks = new Set<object>();
 ks.add(viaView); ks.add(typed); ks.add(p.o); ks.add(field.o);
 const extra = seen.get(typed)+":"+seen.get(p.o)+":"+ks.size+":"+(typeof viaView);
 const more = JSON.stringify(viaView)+":"+Object.keys(viaView).length;
 return more+":"+(first===second)+":"+(second===a)+":"+extra+":"+(viaView===typed)+":"+(p.o===p.arr)+":"+(field.o===p.o)+":"+(viaView===parsed)+":"+(holder.o===a)+":"+(h.o===a)+":"+(h.o===h.arr)+":"+(h.arr===a)+":"+viaFamily+":"+w.repeat+":"+(w.last!==a);
 }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

// A parsed array bridged both ways without any live class: a typed view
// (unknown[]) and an object-root carrier (dynObject keeps the tagArray box).
// equal() already unwraps both; Map and Set keys must agree as well.
func TestObjectRootArrayCarrierCollectionKeyNodeOracle(t *testing.T) {
	source := `class Probe {
 static run():string {
 const raw: any = JSON.parse('[1]');
 const arr: unknown[] = raw;
 const o: object = raw;
 const m = new Map<object,number>();
 m.set(o,1);
 const s = new Set<object>();
 s.add(o); s.add(arr); s.add(raw);
 const xs: object[] = [o];
 return (o===arr)+":"+m.get(arr)+":"+m.get(raw)+":"+s.size+":"+xs.indexOf(arr)+":"+xs.includes(arr);
 }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
