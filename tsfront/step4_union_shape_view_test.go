package tsfront

import "testing"

// frontend.mjs convert(): `/** @type {{readonly value?: {readonly length?:
// number} | null}} */ (expr).value?.length` views a ValueExpr union through
// an anonymous shape whose only field is optional. Members without value
// read undefined; members whose value is a string read its length.
func TestStep4UnionShapeViewNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Meta = {readonly type: string};
export type Expr = (Meta & {readonly e: "chars"; readonly value: string}) | (Meta & {readonly e: "int"; readonly value: number}) | (Meta & {readonly e: "neg"; readonly x: Expr});
`, `/** @param {import("./types.ts").Expr} expr @param {number} len @returns {boolean} */
function hexOk(expr, len) {
  return /** @type {{readonly value?: {readonly length?: number} | null}} */ (expr).value?.length === 2 * len && expr.e === "chars";
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Expr[]} */ const xs = [{e: "chars", value: "ABCD", type: "x"}, {e: "chars", value: "AB", type: "x"}, {e: "int", value: 4, type: "i"}];
    xs.push({e: "neg", x: xs[0], type: "i"});
    return xs.map((x) => String(hexOk(x, 2))).join(",");
  }
}
`, false, true)
}
