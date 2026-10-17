package tsfront

import "testing"

// frontend.mjs table inventory: `let st; ... st.getComponents().map((c) =>
// { const len = cond ? c.type.getLength() : 0; ... {len: 2 * len - 1} })`.
// st is untyped, so len is any; `2 * len` converts it with ToNumber.
func TestStep4AnyArithmeticNodeOracle(t *testing.T) {
	runMjsOracle(t, `class Comp { /** @param {number} n */ constructor(n) { this.n = n; } /** @returns {number} */ getLength() { return this.n; } }
class Struct { /** @returns {Comp[]} */ getComponents() { return [new Comp(3), new Comp(5)]; } }
/** @returns {Struct} */ const parse = () => new Struct();
export class Probe {
  /** @returns {string} */ static run() {
    let st;
    try { st = parse(); } catch { return "x"; }
    return st.getComponents().map((c, i) => {
      const len = i > 0 ? c.getLength() : i === 0 ? c.getLength() : 0;
      return String(2 * len - 1) + "/" + String(len * 2) + "/" + String(len - 1);
    }).join(",");
  }
}
`, false)
}
