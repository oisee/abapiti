package tsfront

import "testing"

// Unconstrained type parameters that only cross the call boundary
// (run: () => T returning T, wrappers of (ctx: C, obj: O) => T) are tagged
// values converted to the instantiated type at each call site.
func TestGenericBoundaryErasureNodeOracle(t *testing.T) {
	runMjsOracle(t, `let depth = 0;
/** @template T @param {string} role @param {() => T} run @returns {T} */
export function withRole(role, run) { depth++; try { return run(); } finally { depth--; } }
/** @template T @param {() => T} run @returns {T} */
export function recorded(run) { return withRole("r", run); }
/** @template C, O, T @param {(ctx: C, obj: O) => T} lower @returns {(ctx: C, obj: O) => T} */
export function wrap(lower) { return (ctx, obj) => withRole("w", () => lower(ctx, obj)); }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {number} */ const n = withRole("a", () => 5 + depth);
    /** @type {string} */ const s = recorded(() => "x" + depth);
    /** @type {{a: number}} */ const o = withRole("b", () => ({a: depth}));
    const f = wrap((/** @type {string} */ c, /** @type {number} */ k) => c + k + depth);
    let thrown = "no";
    try { withRole("c", () => { throw new Error("boom"); }); } catch (e) { thrown = "yes" + depth; }
    return [String(n), s, String(o.a), f("c", 2), thrown, String(depth)].join(",");
  }
}
`, false)
}

// T inside a container (T[]) has no exact tagged view of typed storage: the
// generic stays refused at its location.
func TestGenericContainerParameterRefused(t *testing.T) {
	source := `function append<T>(target: T[], ...arrays: readonly (readonly T[])[]): void { for (const a of arrays) for (const v of a) target.push(v); }
class Probe {static run():number {const xs:number[] = [1]; append(xs, [2]); return xs.length;}}`
	for _, d := range lowerFamilyDiagnostics(t, source) {
		if d.Category == "unsupported-type" && d.Message == "unconstrained type parameter T" {
			return
		}
	}
	t.Fatalf("container type parameter was not refused: %v", lowerFamilyDiagnostics(t, source))
}
