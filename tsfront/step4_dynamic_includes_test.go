package tsfront

import "testing"

// includes/indexOf on an any[] receiver box the search value and compare
// JavaScript values: indexOf uses ===, includes SameValueZero (NaN matches).
func TestStep4DynamicIncludesNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @param {unknown} s @returns {string} */ const upper = (s) => String(s).toUpperCase();
/** @returns {any[]} */ const mixed = () => [1, "A", null, NaN];
export class Probe {
  /** @returns {string} */ static run() {
    const xs = mixed();
    const out = [];
    for (const n of ["a", "1", "x"]) out.push(String(xs.includes(upper(n))) + String(xs.indexOf(upper(n))));
    out.push(String(xs.includes(1)), String(xs.includes(NaN)), String(xs.indexOf(NaN)), String(xs.includes(null)), String(xs.indexOf(null)), String(xs.includes(undefined)));
    return out.join(",");
  }
}
`, false)
}
