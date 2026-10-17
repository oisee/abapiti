package tsfront

import "testing"

// frontend.mjs VALUE table rows: source(inner[0], ctx, to.row) passes a
// subset family value (to.row) to an optional Type parameter.
func TestStep4SubsetOptionalFamilyNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Type = {readonly k: "i"} | {readonly k: "c"; readonly len: number} | {readonly k: "ref"; readonly to: Type} | {readonly k: "table"; readonly row: Scalar};
export type Scalar = Extract<Type, {k: "i"} | {k: "c"}>;
`, `/** @param {string} text @param {import("./types.ts").Type} [outer] @returns {string} */
function source(text, outer) { return text + ":" + (outer === undefined ? "-" : outer.k); }
/** @param {import("./types.ts").Type} to @returns {string} */
function rows(to) {
  if (to.k !== "table") return source("x");
  return source("r", to.row);
}
export class Probe {
  /** @returns {string} */ static run() {
    return [rows({k: "table", row: {k: "c", len: 1}}), rows({k: "table", row: {k: "i"}}), rows({k: "i"})].join(",");
  }
}
`, false, true)
}
