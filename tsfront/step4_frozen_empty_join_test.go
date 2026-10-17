package tsfront

import "testing"

// OSG frontend-queries.mjs suppliedAnswer(): set?.size ? providers :
// emptyProviders, where emptyProviders = Object.freeze([]) (readonly never[])
// and the parameter is readonly QueryScalar[] (string | number | boolean).
func TestStep4FrozenEmptyArrayJoinNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type QueryScalar = string | number | boolean;
`, `/** @type {string[]} */ const log = [];
const emptyProviders = Object.freeze([]);
/** @param {string} kind @param {readonly import("./types.ts").QueryScalar[]} result @returns {void} */
function semanticQuery(kind, result) { log.push(kind + "=" + result.map((r) => String(r)).join("|")); }
/** @param {Set<string> | undefined} set @param {readonly string[]} providers @returns {number} */
function suppliedAnswer(set, providers) {
  semanticQuery("supplied", set?.size ? providers : emptyProviders);
  semanticQuery("direct", providers);
  return set?.size ?? -1;
}
export class Probe {
  /** @returns {string} */ static run() {
    const a = suppliedAnswer(new Set(["x"]), ["p", "q"]);
    const b = suppliedAnswer(undefined, ["r"]);
    const c = suppliedAnswer(new Set(), ["s"]);
    return log.join(",") + ":" + a + b + c + ":" + emptyProviders.length;
  }
}
`, false, true)
}
