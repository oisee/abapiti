package tsfront

import (
	"strings"
	"testing"
)

// Map.prototype.clear (frontend.mjs session.classes.clear(),
// LOCAL_DEFS.clear()) empties the map in place: every alias observes it,
// size is 0, lookups miss, and later sets restart insertion order.
func TestGogenA33MapClearNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @type {Map<string, number>} */ const SHARED = new Map();
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Map<string, number>} */ const m = new Map([["b", 1], ["a", 2], ["c", 3]]);
    const alias = m;
    const before = m.size;
    m.clear();
    m.set("z", 9);
    m.set("b", 4);
    SHARED.set("x", 1);
    SHARED.set("y", 2);
    SHARED.clear();
    SHARED.clear();
    SHARED.set("w", 5);
    return before + ":" + alias.size + ":" + alias.has("a") + ":" + String(alias.get("c")) + ":" + [...alias.keys()].join(",") + ":" + [...alias.values()].join(",") + ":" + [...SHARED.keys()].join(",") + ":" + SHARED.size;
  }
}
`, true)
}

// frontend.mjs (OSG dd94a6a9) destructures a typed inline lazy require in a
// block and holds the functions in a Pick-typed optional record:
// `const {f, g} = /** @type {typeof import("./m.mjs")} */ (require("./m.mjs"));`
func TestGogenA33DestructuredInlineRequireNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {require} from "./base.mjs";
/** @param {number} n @returns {number} */ function plain(n) { return n + 100; }
export class Probe {
  /** @param {boolean} on @returns {string} */
  static pick(on) {
    /** @type {Pick<typeof import("./lazy.mjs"), "twice" | "thrice"> | undefined} */ let lazy;
    if (on) {
      const {twice, thrice} = /** @type {typeof import("./lazy.mjs")} */ (require("./lazy.mjs"));
      lazy = {twice, thrice};
    }
    const a = lazy ? lazy.twice(3) : -1;
    const f = lazy ? lazy.thrice : plain;
    return a + "," + f(4);
  }
  /** @returns {string} */
  static run() {
    return Probe.pick(true) + "|" + Probe.pick(false);
  }
}
`, "6,12|-1,104\n", map[string]string{
		"base.mjs": `import {createRequire} from "node:module";
const require = createRequire(import.meta.url);
export {require};
`,
		"lazy.mjs": `/** @param {number} n @returns {number} */ export function twice(n) { return n * 2; }
/** @param {number} n @returns {number} */ export function thrice(n) { return n * 3; }
`,
	})
}

// frontend.mjs `if (session) session.cdsInputs = cdsIdentity;` writes a key
// the mixed index record (CacheSession: [key: string]: unknown plus named
// slots) does not declare.
func TestGogenA33UndeclaredIndexWriteNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Session {[key: string]: unknown; files: string[]; classes: Map<string, number>;}
class Probe {
  static put(s: Partial<Session> | null, v: string[]): void { if (s) { s.cdsInputs = v; s["n"] = v.length; } }
  static run(): string {
    const a: Partial<Session> = {files: ["f"]};
    Probe.put(a, ["x", "y"]);
    Probe.put(null, []);
    Probe.put(a, ["z"]);
    const b: Session = {files: [], classes: new Map()};
    Probe.put(b, []);
    const v = a["cdsInputs"] as string[];
    return Object.keys(a).join(",") + ":" + typeof a.cdsInputs + ":" + Object.keys(b).join(",") + ":" + v.join("+") + ":" + String(a.cdsInputs) + ":" + String(a.n);
  }
}`, "")
}

// frontend.mjs `Object.assign(program, {faeN: 0, localDefs: new Map()})` and
// `Object.assign(program, {amcChannels: f(), ownershipSafety: ...})`: the
// target is evaluated first, then every value left to right, then each key
// is set in literal order; the call returns the target itself.
func TestGogenA33ObjectAssignTypedRecordNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Prog { name: string; faeN?: number; defs?: Map<string, number>; tag?: string; log: string[] }
`, `/** @param {import("./types.ts").Prog} p @param {string} s @returns {string} */
function note(p, s) { p.log.push(s + ":" + String(p.faeN) + ":" + (p.defs ? p.defs.size : -1)); return s; }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Prog} */ const p = {name: "p", log: []};
    const alias = p;
    const tag = "t";
    Object.assign((note(p, "target"), p), {faeN: 7, tag, defs: new Map([[note(p, "value"), 1]])});
    Object.assign(p, {faeN: (p.faeN ?? 0) + 1});
    return (p === alias) + ":" + p.faeN + ":" + p.tag + ":" + (p.defs ? [...p.defs.keys()].join(",") : "none") + ":" + p.log.join("|");
  }
}
`, true)
}

// A used result (target & literal is another record shape), an undeclared
// key, an index-like key or a spread stays a located refusal.
func TestGogenA33ObjectAssignTypedRecordRefusals(t *testing.T) {
	for _, call := range []string{`const r = Object.assign(p, {n: 1}); return r.n;`, `Object.assign(p, {other: 1}); return 0;`, `Object.assign(p, {"1": 1}); return 0;`, `Object.assign(p, {...{n: 2}}); return 0;`} {
		p := integerProbe(t, `interface Prog { name: string; n?: number; "1"?: number }
export class Probe { run(p: Prog) { `+call+` } }`)
		_, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range ds {
			found = found || d.Category == "unsupported-call" && strings.Contains(d.Message, "Object.assign") && strings.Contains(d.Loc, "probe.ts:")
		}
		if !found {
			t.Errorf("%s: missing located refusal: %v", call, ds)
		}
	}
}

// frontend.mjs `session ? (session.ownershipSafety ??= sourceOwnershipSafety(reg)) : ...`:
// ??= on a key the index-signature session does not declare reads it once,
// evaluates the right side only when it is undefined or null, stores and
// yields it.
func TestGogenA33NullishAssignIndexKeyNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Session { [key: string]: unknown; files: string[]; known?: ReadonlyMap<string, number> }
export interface Prog { name: string; safety?: ReadonlyMap<string, number> }
`, `let calls = 0;
/** @param {string} tag @returns {ReadonlyMap<string, number>} */
function make(tag) { calls++; return new Map([[tag, calls]]); }
/** @param {import("./types.ts").Prog} p @param {Partial<import("./types.ts").Session> | null} s */
function fill(p, s) { Object.assign(p, {safety: s ? (s.safety ??= make("s")) : make("n")}); }
/** @param {import("./types.ts").Prog} p @param {Partial<import("./types.ts").Session> | null} s */
function fillKnown(p, s) { Object.assign(p, {safety: s ? (s.known ??= make("k")) : make("m")}); }
/** @param {import("./types.ts").Prog} p @returns {string} */
function show(p) { return p.safety ? [...p.safety].map(([k, v]) => k + v).join(",") : "none"; }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Partial<import("./types.ts").Session>} */ const s = {files: []};
    /** @type {import("./types.ts").Prog} */ const p = {name: "p"};
    fill(p, s); const a = show(p);
    fill(p, s); const b = show(p);
    fill(p, null); const c = show(p);
    s.safety = null;
    fill(p, s); const d = show(p);
    /** @type {Partial<import("./types.ts").Session>} */ const t = {};
    const e = String(t.k ??= 4) + ":" + String(t.k ??= 5) + ":" + Object.keys(t).join(",");
    fillKnown(p, s); const f = show(p);
    fillKnown(p, s); const g = show(p);
    fillKnown(p, null); const h = show(p);
    return [a, b, c, d, e, f, g, h, String(calls)].join("|");
  }
}
`, true)
}
