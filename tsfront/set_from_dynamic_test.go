package tsfront

import "testing"

// The step-4 driver: `const inputs = JSON.parse(...)` is untyped;
// `/** @type {Set<string>} */ const skipped = new Set(inputs.skippedPaths)`
// iterates the parsed array; new Set(undefined) is empty.
func TestSetFromDynamicNodeOracle(t *testing.T) {
	runMjsOracle(t, `const inputs = JSON.parse('{"skippedPaths":["b","a","b"],"includeTests":["t"]}');
/** @type {Set<string>} */ const skipped = new Set(inputs.skippedPaths);
/** @param {{tests: Set<string>}} o @returns {number} */ function count(o) { return o.tests.size; }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Set<string>} */ const none = new Set(inputs.missing);
    const n = count({tests: /** @type {Set<string>} */ (new Set(inputs.includeTests))});
    return [...skipped].join("") + ":" + skipped.has("a") + ":" + none.size + ":" + n;
  }
}
`, true)
}
