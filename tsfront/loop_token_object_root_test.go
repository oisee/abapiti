package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// Opaque LoopToken fields (type LoopToken = object, ir.ts) hold plain {}
// records: tagObject carriers. Reading such a field through a family or live
// view must keep the record itself, so === and Set/Map keys keep JS reference
// identity across every read path: the same record compares equal, a fresh
// record stays unequal, and a view of the record is the same collection key.
// gogen's Stmt walkers depend on exactly this shape: frontend.mjs builds the
// loop arms as spread records with token: {}, emit-go.mjs reads st.token back
// through the family view.

func TestLoopTokenObjectRootFamilyViewIdentityNodeOracle(t *testing.T) {
	source := `type LoopToken = object;
 type Stmt = {k:"loop"; token:LoopToken; body:Stmt[]} | {k:"delete_current"; token:LoopToken};
 type LoopArm = {k:"loop"; token:LoopToken; body:Stmt[]};
 class Walker {
 deleted:Set<LoopToken> = new Set();
 names:Map<LoopToken,string> = new Map();
 last:LoopToken|undefined = undefined;
 count:number = 0;
 repeat:number = 0;
 walk(st:Stmt):void {
 if (st.k==="loop") {this.names.set(st.token,"loop");for (const inner of st.body) this.walk(inner);return;}
 const token = st.token;
 this.deleted.add(token);
 if (this.names.get(token)==="loop") this.count++;
 if (this.last===token) this.repeat++;
 this.last = token;
 }
 }
 class Probe {
 static mk(token:LoopToken, body:Stmt[]):Stmt {const base = {k:"loop" as const, token, body};const arm:LoopArm = {...base};return arm;}
 static run():string {
 const token:LoopToken = {};
 const other:LoopToken = {};
 const st = Probe.mk(token,[{k:"delete_current", token},{k:"delete_current", token},{k:"delete_current", token:other}]);
 const w = new Walker();
 w.walk(st);
 return w.deleted.size+":"+w.count+":"+w.repeat+":"+(w.names.get(token)==="loop")+":"+w.deleted.has(token)+":"+w.deleted.has(other);
 }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	// Recursive structural families keep their explicit ABAP refusal.
	if _, _, err := emitABAPNamed(t, p); err != nil {
		if !strings.Contains(err.Error(), "probe.ts:") || !strings.Contains(err.Error(), "ABAP") {
			t.Fatal(err)
		}
		t.Logf("located ABAP refusal: %v", err)
	}
}

// The same walker typed in place with JSDoc, like gogen's .mjs sources: the
// loop arm is a spread record, the token is a {} literal, Map keys and ===
// compare through the family view. (this.n++ on JS fields is a separate
// pre-existing lowering gap; this probe stays with n = n + 1.)
func TestLoopTokenObjectRootJSDocMjsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type LoopToken = object;
`, `/** @typedef {import("./types.ts").LoopToken} LoopToken */
/** @typedef {{k: "loop", token: LoopToken, body: Stmt[]} | {k: "delete_current", token: LoopToken}} Stmt */
export class Walker {
  constructor() {
    /** @type {Map<LoopToken,string>} */ this.names = new Map();
    /** @type {LoopToken} */ this.last = {};
    this.count = 0;
    this.repeat = 0;
  }
  /** @param {Stmt} st */ walk(st) {
    if (st.k === "loop") { this.names.set(st.token, "loop"); for (const inner of st.body) this.walk(inner); return; }
    const token = st.token;
    if (this.names.get(token) === "loop") this.count = this.count + 1;
    if (this.last === token) this.repeat = this.repeat + 1;
    this.last = token;
  }
}
export class Probe {
  /** @param {LoopToken} token @param {Stmt[]} body @returns {Stmt} */ static mk(token, body) { const base = {k: "loop", token, body}; /** @type {{k: "loop", token: LoopToken, body: Stmt[]}} */ const arm = {...base}; return arm; }
  static run() {
    /** @type {LoopToken} */ const token = {};
    /** @type {LoopToken} */ const other = {};
    const st = Probe.mk(token, [{k: "delete_current", token}, {k: "delete_current", token}, {k: "delete_current", token: other}]);
    const w = new Walker();
    w.walk(st);
    return w.names.size + ":" + w.count + ":" + w.repeat + ":" + (w.names.get(token) === "loop");
  }
}
`, false)
}

// A live view field of the object root keeps the parsed record's identity
// without any family: two reads compare ===, a fresh {} stays unequal, and the
// record is a stable Map key on both Go and the ABAP host.
func TestLoopTokenObjectRootLiveFieldIdentityNodeOracle(t *testing.T) {
	source := `interface Holder { link: object; }
 class Probe {
 static run():string {
 const h:Holder = JSON.parse('{"link":{}}');
 const a = h.link;
 const b = h.link;
 const seen = new Map<object,string>();
 seen.set(a,"first");
 return (a===b)+":"+(seen.get(b)==="first")+":"+(a!=={});
 }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}
