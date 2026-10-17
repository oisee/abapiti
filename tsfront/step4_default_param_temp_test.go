package tsfront

import "testing"

// OSG frontend-expressions-calls.mjs source(node, ctx, outer, hint = outer):
// the default value's conversion into the parameter's family needs a
// temporary, which must live in the branch that applies the default.
func TestStep4DefaultParameterTemporaryNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `interface Meta { readonly ddic?: string }
export type Type = Meta & ({ readonly k: "i" } | { readonly k: "c"; readonly len: number } | { readonly k: "p"; readonly len: number; readonly dec: number } | { readonly k: "table"; readonly row: Type });
`, `/** @param {import("./types.ts").Type} t @returns {string} */
function show(t) { return t.k === "i" ? "i" : t.k === "c" ? "c" + t.len : t.k === "table" ? "t(" + show(t.row) + ")" : "p" + t.len + "." + t.dec; }
/** @param {string} x @param {import("./types.ts").Type} [outer] @param {import("./types.ts").Type} [hint] @returns {string} */
function source(x, outer, hint = outer) {
  return x + ":" + (outer ? show(outer) : "-") + ":" + (hint ? show(hint) : "-");
}
export class Probe {
  /** @returns {string} */ static run() {
    return [source("a"), source("b", {k: "c", len: 2}), source("c", {k: "i"}, {k: "p", len: 3, dec: 1}), source("d", {k: "table", row: {k: "i"}})].join(" ");
  }
}
`, false, true)
}
