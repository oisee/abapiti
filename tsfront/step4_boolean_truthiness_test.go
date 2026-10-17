package tsfront

import "testing"

// frontend-inventory.mjs hasBuiltArtifacts: `Boolean(obj.syntaxResult ||
// obj.def) || obj.isDirty?.() === false` in a boolean-returning arrow. The
// outer boolean context must not type the inner `a || b` as boolean: it is
// the first truthy operand (a string, an object), which Boolean() converts by
// truthiness.
func TestStep4BooleanTruthinessNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @param {unknown} a @param {unknown} b @param {boolean} c @returns {boolean} */
const built = (a, b, c) => Boolean(a || b) || c === false;
export class Probe {
  /** @returns {string} */ static run() {
    return [built("ok", undefined, true), built(undefined, {x: 1}, true), built(0, "", true), built(undefined, null, false), built("", [], true)].map((b) => String(b)).join(",");
  }
}
`, false)
}
