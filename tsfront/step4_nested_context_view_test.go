package tsfront

import "testing"

// frontend.mjs methodSignature() passes its CompilationContext {reg, program:
// FrontendProgram} to frontend-queries.mjs suppliedSignature(ctx:
// QueryContext), where QueryContext is {reg, program: QueryProgram} and
// FrontendProgram has every QueryProgram field. The callee reads
// ctx.program.sigs/supplied off the same live records.
func TestStep4NestedContextViewNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Registry { name: string }
export interface QueryProgram { reg: Registry; supplied: Map<string, Set<string>>; caught?: Map<string, boolean> }
export interface QueryContext { reg: Registry; program: QueryProgram }
export interface FrontendProgram { reg: Registry; supplied: Map<string, Set<string>>; caught?: Map<string, boolean>; sigs: Map<string, string>; wanted: Set<string> }
export interface CompilationContext { reg: Registry; program: FrontendProgram }
`, `/** @param {import("./types.ts").QueryContext} ctx @param {string} key @returns {string} */
function suppliedSignature(ctx, key) {
  const set = ctx.program.supplied.get(key);
  ctx.program.supplied.set(key + "!", new Set(["Z"]));
  return ctx.reg.name + ":" + (set ? [...set].join("") : "-") + ":" + String(ctx.program.caught?.get(key) ?? false);
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Registry} */ const reg = {name: "r"};
    /** @type {import("./types.ts").FrontendProgram} */ const program = {reg, supplied: new Map([["a", new Set(["X", "Y"])]]), sigs: new Map(), wanted: new Set()};
    /** @type {import("./types.ts").CompilationContext} */ const ctx = {reg, program};
    const out = suppliedSignature(ctx, "a") + "," + suppliedSignature(ctx, "b");
    return out + "," + String(program.supplied.size);
  }
}
`, false, true)
}
