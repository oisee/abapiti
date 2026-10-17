package tsfront

import "testing"

// frontend.mjs declares a module function named `constructor` (the ABAP
// constructor expression) and calls it for its ValueExpr result.
func TestStep4ConstructorNamedFunctionNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @param {string} c @param {number} [n] @returns {{e: string, n: number}} */ function constructor(c, n) {
  return {e: c, n: n ?? 0};
}
/** @param {string} c @returns {string} */ const use = (c) => { const v = constructor(c, 2); return v.e + v.n; };
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {string[]} */ const out = [];
    for (const c of ["a", "b"]) out.push(use(c), constructor(c).e, String(constructor(c).n));
    return out.join(",");
  }
}
`, true)
}
