package tsfront

import "testing"

// `/** @type {B} */ (/** @type {unknown} */ (a))` (the step-4 driver's view
// of compileProgram's SessionProgram as ir.ts Program): the object reaches B
// through the checked erased-reference view, identity and mutations kept.
func TestUnknownDoubleAssertionNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Session {reg: string; sigs: Map<string, string>}
export interface Program {readonly classes: string[]; reg: string; sigs?: Map<string, string>; tag?: string}
`, `/** @returns {import("./types.ts").Session} */ function compile() {
  /** @type {import("./types.ts").Program} */ const p = {classes: ["a", "b"], reg: "r", sigs: new Map()};
  return /** @type {import("./types.ts").Session} */ (/** @type {unknown} */ (p));
}
/** @param {import("./types.ts").Program} p @returns {string} */ function collect(p) {
  p.tag = "seen";
  return p.classes.join("+") + ":" + p.reg;
}
export class Probe {
  /** @returns {string} */ static run() {
    const program = /** @type {import("./types.ts").Program} */ (/** @type {unknown} */ (compile()));
    return collect(program) + ":" + program.tag;
  }
}
`, true)
}
