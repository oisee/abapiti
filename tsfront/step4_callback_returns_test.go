package tsfront

import "testing"

// frontend.mjs tryStatement: `/** @type {FrontendCatch[]} */ const catches =
// node.findDirectStructures(Catch).map((c) => { ... if (...) return {...};
// ... return {...}; })`. The callback's inferred result is the union of its
// literals; the declared array gives their type.
func TestStep4CallbackReturnsElementNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Catch { readonly classes: readonly string[]; readonly into: string | null; readonly kind: 'exc' | 'ref' | null }
type Shape<T> = T extends readonly (infer Item)[] ? readonly Shape<Item>[] : T extends object ? {readonly [K in keyof T]: Shape<T[K]>} : T;
export type FrontendCatch = Shape<Catch>;
`, `export class Probe {
  /** @returns {string} */ static run() {
    /** @type {string[]} */ const seen = [];
    /** @type {import("./types.ts").FrontendCatch[]} */ const catches = [["A"], ["B", "C"], []].map((names) => {
      if (names.some((n) => seen.includes(n))) throw new Error("dup");
      seen.push(...names);
      if (names.length === 1) return {classes: names, into: "X", kind: "exc"};
      return {classes: names, into: null, kind: null};
    });
    /** @type {import("./types.ts").Catch[]} */ const plain = [1, 2].map((n) => {
      if (n > 1) return {classes: [String(n)], into: null, kind: "ref"};
      return {classes: [], into: "Y", kind: null};
    });
    return [...catches, ...plain].map((c) => c.classes.join("+") + ":" + String(c.into) + ":" + String(c.kind)).join(",");
  }
}
`, false)
}
