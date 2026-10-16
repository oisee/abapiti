package tsfront

import "testing"

// JSON.stringify with a replacer function (called for every key, the root
// "" first, in serialization order; undefined drops a property or the whole
// result), with a replacer key list, and with space.
func TestJSONReplacerNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @typedef {{e: string, type: {k: string}, kids: {e: string, type: {k: string}}[]}} Node */
/** @param {Node} x @returns {string | undefined} */
const bare = (x) => JSON.stringify(x, (k, v) => (k === "type" ? undefined : v));
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {string[]} */ const seen = [];
    /** @type {Node} */ const n = {e: "a", type: {k: "i"}, kids: [{e: "b", type: {k: "c"}}]};
    const logged = JSON.stringify(n, (k, v) => { seen.push(k === "" ? "<root>" : k); return v; });
    const keyed = JSON.stringify(n, ["kids", "e"]);
    const spaced = JSON.stringify(n, (k, v) => (k === "k" ? v + "!" : v), 2);
    const swapped = JSON.stringify(n, (k, v) => (k === "e" ? 7 : v));
    return [String(bare(n)), String(bare(n) === bare({e: "a", type: {k: "z"}, kids: [{e: "b", type: {k: "y"}}]})), String(logged), seen.join("|"), String(keyed), String(spaced), (JSON.stringify(n, (k, v) => (k === "" ? undefined : v)) === undefined ? "undefined" : "defined"), String(swapped)].join("\n");
  }
}
`, false)
}
