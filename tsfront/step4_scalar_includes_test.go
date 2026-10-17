package tsfront

import "testing"

// string[].includes(x) where x is an optional field (frontend-expressions-
// calls.mjs ["+", "-"].includes(parts[1].op)) or an untyped JS value
// (frontend.mjs sortable(t): [...].includes(t.k)): non-strings are not found.
func TestStep4ScalarIncludesNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Item { readonly op?: string }
`, `/** @param {import("./types.ts").Item} it @returns {boolean} */
const arith = (it) => ["+", "-"].includes(it.op);
const sortable = (t) => ["i", "c", "string"].includes(t.k);
export class Probe {
  /** @returns {string} */ static run() {
    return [arith({op: "+"}), arith({op: "*"}), arith({}), sortable({k: "c"}), sortable({k: 1}), sortable({k: undefined}), sortable({})].map((b) => String(b)).join(",");
  }
}
`, false, true)
}
