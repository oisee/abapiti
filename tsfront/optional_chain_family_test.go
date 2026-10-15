package tsfront

import "testing"

// Optional chains over optional family values: t?.k on Ty | undefined, a
// later link (t?.k.concat(...)) short-circuits with its side effects,
// m.body?.[tick()] on an optional family array, and flow narrowing after
// t?.k === 'table' proving t present.
func TestOptionalChainFamilyNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Meta = {readonly qname?: string};
export type Ty = Meta & ({readonly k: 'i'} | {readonly k: 'table'; readonly row: Ty} | {readonly k: 'struct'; readonly go: string});
export type Stmt = {readonly s: 'call'; readonly fn: string} | {readonly s: 'seq'; readonly body: readonly Stmt[]};
export type Method = {name: string; body?: readonly Stmt[]};
export type Cls = {name: string; methods: Method[]};
export type Item = {name: string, size: number};
`, `/** @typedef {import("./types.ts").Ty} Ty */
/** @typedef {import("./types.ts").Method} Method */
/** @typedef {import("./types.ts").Cls} Cls */
let calls = 0;
/** @returns {number} */ function tick() { calls++; return 0; }
export class Probe {
  /** @param {Ty | undefined} t @returns {boolean} */ static composite(t) { return t?.k === "table" || t?.k === "struct"; }
  /** @param {Ty | undefined} t @returns {string} */ static row(t) { return String(t?.k === "table" ? t.row.k : t?.qname); }
  /** @param {Method} m @param {string} fn @returns {boolean} */ static first(m, fn) { const s = m.body?.[tick()]; return s?.s === "call" && s.fn === fn; }
  /** @param {Cls[]} classes @param {string} cls @param {string} meth @returns {string} */ static find(classes, cls, meth) { return String(classes.find((c) => c.name === cls)?.methods.find((m) => m.name === meth)?.name); }
  /** @param {Ty | undefined} t @returns {string} */ static chain(t) { return String(t?.k.concat(String(tick()))); }
  /** @returns {string} */ static run() {
    /** @type {Ty} */ const tab = {k: "table", row: {k: "i"}};
    /** @type {Method} */ const m1 = {name: "a", body: [{s: "call", fn: "f"}]};
    /** @type {Method} */ const m2 = {name: "b"};
    /** @type {Cls[]} */ const cs = [{name: "C", methods: [m1, m2]}];
    /** @type {string[]} */ const out = [String(Probe.composite(undefined)), String(Probe.composite(tab)), String(Probe.composite({k: "struct", go: "G"})), String(Probe.composite({k: "i"})), String(Probe.row(tab)), String(Probe.row(undefined)), String(Probe.row({k: "i", qname: "Q"})), String(Probe.first(m1, "f")), String(Probe.first(m2, "f")), String(Probe.find(cs, "C", "b")), String(Probe.find(cs, "D", "b")), String(Probe.find(cs, "C", "z")), Probe.chain(undefined), Probe.chain(tab), String(calls)];
    return out.join(",");
  }
}
`, false)
}
