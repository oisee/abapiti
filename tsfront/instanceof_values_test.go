package tsfront

import "testing"

// instanceof against class values: a constructor-typed parameter, a
// typeof-union local (subclass chains included), and instanceof Map / Set /
// Array.isArray on an untyped value (a record is not a Map).
func TestInstanceOfClassValueNodeOracle(t *testing.T) {
	// A recursive family with an empty-record field registers the {} shape a
	// constructor type must not be mistaken for.
	runMjsOracleTypes(t, "export type Tree = {k: 'leaf'; meta: {}} | {k: 'node'; kids: Tree[]};\n", `/** @typedef {import("./types.ts").Tree} Tree */
/** @param {Tree} t @returns {number} */ export function leaves(t) { return t.k === "leaf" ? 1 : t.kids.reduce((s, k) => s + leaves(k), 0); }
export class Node0 { /** @returns {string} */ kind() { return "node"; } }
export class Expr extends Node0 {}
export class Source extends Expr {}
export class Stmt extends Node0 {}
/** @param {Node0[]} nodes @param {new () => Node0} type @returns {number} */
function count(nodes, type) { let n = 0; for (const x of nodes) if (x instanceof type) n++; return n; }
/** @typedef {typeof Expr | typeof Stmt} NodeClass */
/** @param {Node0} n @param {NodeClass} cls @returns {boolean} */
const is = (n, cls) => n instanceof cls;
/** @param {any} v @returns {string} */
const kindOf = (v) => v instanceof Map ? "map" : v instanceof Set ? "set" : Array.isArray(v) ? "array" : typeof v;
export class Probe {
  /** @returns {string} */ static run() {
    const nodes = [new Expr(), new Source(), new Stmt(), new Node0()];
    /** @type {Map<string, number>} */ const m = new Map(); m.set("a", 1);
    /** @type {Record<string, number>} */ const rec = {a: 1};
    return [String(count(nodes, Expr)), String(count(nodes, Source)), String(count(nodes, Stmt)), String(count(nodes, Node0)), String(is(nodes[1], Expr)), String(is(nodes[2], Expr)), String(is(nodes[2], Stmt)), String(kindOf(m)), String(kindOf(new Set([1, 2]))), String(kindOf([1, 2, 3])), String(kindOf(rec)), String(kindOf("s")), String(kindOf(undefined)), String(leaves({k: "node", kids: [{k: "leaf", meta: {}}, {k: "leaf", meta: {}}]}))].join(",");
  }
}
`, false)
}
