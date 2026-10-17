package tsfront

import "testing"

// frontend.mjs table inventory: `let key = []; try { key = view ? [] :
// (table?.listKeys?.(reg) ?? []).map(upper); } catch { key = []; }`, then
// `key.includes(upper(c.name))` inside a callback. The checker leaves key
// any[] at every reference; every assigned array is a string[].
func TestStep4EvolvingArrayAssignedNodeOracle(t *testing.T) {
	runMjsOracle(t, `class Tab { /** @returns {string[]} */ listKeys() { return ["a", "b"]; } }
/** @param {unknown} s @returns {string} */ const upper = (s) => String(s).toUpperCase();
/** @param {boolean} view */
const f = (view) => {
  /** @type {Tab | undefined} */ const table = view ? undefined : new Tab();
  let key = [];
  try { key = view ? [] : (table?.listKeys?.() ?? []).map(upper); } catch { key = []; }
  return ["a", "B", "c", "1"].map((n) => String(key.includes(upper(n))) + key.length).join(",");
};
export class Probe {
  /** @returns {string} */ static run() {
    return f(false) + "|" + f(true);
  }
}
`, false)
}
