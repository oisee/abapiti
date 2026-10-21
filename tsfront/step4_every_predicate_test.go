package tsfront

import "testing"

// OSG frontend-expressions-calls.mjs source(): after
// leaves.every((t) => t.k === "x") (an inferred type predicate) narrows
// leaves, leaves.reduce((max, t) => Math.max(max, t.len), -Infinity) reads a
// field of the one narrowed family member.
func TestStep4EveryPredicateNarrowsArrayNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `interface Meta { readonly ddic?: string }
export type Type = Meta & ({ readonly k: "i" } | { readonly k: "x"; readonly len: number } | { readonly k: "xstring" } | { readonly k: "table"; readonly row: Type });
`, `/** @param {import("./types.ts").Type[]} leaves @returns {string} */
function width(leaves) {
  if (!leaves.every((t) => t.k === "x" || t.k === "xstring")) return "mixed";
  if (leaves.every((t) => t.k === "x")) {
    return "x" + leaves.reduce((max, t) => Math.max(max, t.len), -Infinity);
  }
  return "xs";
}
export class Probe {
  /** @returns {string} */ static run() {
    return [width([{k: "x", len: 2}, {k: "x", len: 5}]), width([{k: "x", len: 1}, {k: "xstring"}]), width([{k: "i"}]), width([])].join(" ");
  }
}
`, false, true)
}
