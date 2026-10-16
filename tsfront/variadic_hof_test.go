package tsfront

import "testing"

// A generic higher-order function whose callback parameter carries a
// variadic tuple type parameter A (gogen's artifactLowering) accepts a
// function expression with narrower leading parameters and fixed extra ones
// (a callback, a nullable session); the returned wrapper forwards ...args.
func TestVariadicTupleHOFNodeOracle(t *testing.T) {
	types := `export interface Reg {name: string}
export interface RegCtx {reg: Reg}
export interface LowerCtx extends RegCtx {n: number}
export interface ArtObj {getName(): string}
export type ArtInput = ArtObj | {obj: ArtObj};
export interface ClassObj {getName(): string; obj?: ArtObj}
export type ClassLowering = (ctx: LowerCtx, obj: ClassObj) => string;
export interface Session {hits: number}
`
	runMjsOracleTypes(t, types, `/** @template T @param {import("./types.ts").RegCtx} ctx @param {import("./types.ts").ArtInput} obj @param {() => T} run @returns {T} */ function withArtifact(ctx, obj, run) {
  return run();
}
/** @template {import("./types.ts").RegCtx} C @template {import("./types.ts").ArtInput} O @template {unknown[]} A @template T @param {(ctx: C, obj: O, ...args: A) => T} lower @returns {(ctx: C, obj: O, ...args: A) => T} */ function artifactLowering(lower) {
  return (ctx, obj, ...args) => withArtifact(ctx, obj, () => lower(ctx, obj, ...args));
}
const compile = artifactLowering(/** @param {import("./types.ts").LowerCtx} ctx @param {import("./types.ts").ClassObj} obj @param {import("./types.ts").ClassLowering} lower @param {import("./types.ts").Session | null | undefined} session @returns {string} */ function compile(ctx, obj, lower, session) {
  if (session) session.hits++;
  return lower(ctx, obj) + ":" + ctx.reg.name + ":" + (session ? session.hits : "none");
});
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").ClassLowering} */ const lower = (ctx, obj) => obj.getName() + ctx.n;
    const obj = {getName: () => "zcl"};
    const ctx = {reg: {name: "r"}, n: 7};
    /** @type {import("./types.ts").Session} */ const s = {hits: 1};
    return [compile(ctx, obj, lower, s), compile(ctx, obj, lower, null), compile(ctx, obj, lower, undefined), String(s.hits)].join(" ");
  }
}
`, false)
}
