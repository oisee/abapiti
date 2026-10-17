package tsfront

import "testing"

// An unannotated module-level `new Map([...])` (gogen's TRANSPILER_MEANING,
// AMC_HOST, P_FN) builds its entries in pending statements; the module
// initializer runs them before the assignment.
func TestModuleMapInitializerNodeOracle(t *testing.T) {
	runMjsOracle(t, `const MEANING = new Map([
  ["/UI2/CL_JSON=>_DESERIALIZE", "generic"],
  ["B", "b"],
]);
const HOST = new Map([
  ["k", {op: "send", args: {MESSAGE: "I_MESSAGE"}}],
]);
export class Probe {
  /** @returns {string} */ static run() {
    return MEANING.get("B") + "," + MEANING.size + "," + HOST.get("k")?.op + "," + HOST.get("k")?.args.MESSAGE + "," + [...MEANING.keys()].join("|");
  }
}
`, false)
}
