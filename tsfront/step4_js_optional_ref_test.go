package tsfront

import "testing"

// Unchecked JavaScript passes or returns a possibly-undefined reference
// (Map.get) where JSDoc declares a present one (frontend.mjs nativeArgs(
// k.args), return program.interfaceAttrs.get(intf)): the value flows on.
var step4JSOptionalRef = [2]string{`export class Stmt { readonly name: string; constructor(name: string) { this.name = name; } }
export interface Program { readonly attrs: Map<string, Stmt[]>; readonly stmts: Map<string, Stmt> }
`, `import {Stmt} from "./types.ts";
/** @param {readonly string[]} specs @returns {string} */
function join(specs) { return specs.join("+"); }
/** @param {Stmt} s @returns {string} */
function nameOf(s) { return s.name; }
/** @param {import("./types.ts").Program} p @param {string} k @returns {Stmt[]} */
function attrs(p, k) { return p.attrs.get(k); }
/** @param {import("./types.ts").Program} p @param {string} k @returns {string} */
function first(p, k) { if (!p.stmts.has(k)) return "-"; return nameOf(p.stmts.get(k)); }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Map<string, string[]>} */ const m = new Map([["a", ["x", "y"]]]);
    /** @type {import("./types.ts").Program} */ const p = {attrs: new Map([["a", [new Stmt("s")]]]), stmts: new Map([["b", new Stmt("t")]])};
    return join(m.get("a")) + ":" + attrs(p, "a").length + ":" + String(attrs(p, "z") === undefined) + ":" + first(p, "b") + first(p, "c");
  }
}
`}

func TestStep4JSOptionalReferenceNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, step4JSOptionalRef[0], step4JSOptionalRef[1], false, true)
}
