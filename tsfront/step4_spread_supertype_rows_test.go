package tsfront

import "testing"

// frontend.mjs withSupplied(): `return [...params, ...hidden]` where params
// is FrontendParam[] (Param plus optional fields) and hidden is Param[]. The
// Param rows are FrontendParam views without the extra optional fields.
func TestStep4SpreadSupertypeRowsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Field { readonly name: string; readonly type: string }
export interface Param extends Field { readonly dir?: "importing" | "exporting"; readonly optional?: boolean; readonly suppliedOf?: string }
export interface FrontendParam extends Param { readonly defaultOwner?: string }
`, `/** @param {Set<string> | undefined} set @param {readonly import("./types.ts").FrontendParam[]} params @returns {readonly import("./types.ts").FrontendParam[]} */
function withSupplied(set, params) {
  if (!set) return params;
  /** @type {import("./types.ts").Param[]} */ const hidden = params.filter((p) => set.has(p.name)).map((p) => ({name: "\0" + p.name, dir: "importing", type: "c", optional: true, suppliedOf: p.name}));
  return [...params, ...hidden];
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").FrontendParam[]} */ const ps = [{name: "A", type: "i", defaultOwner: "X"}, {name: "B", type: "c", dir: "exporting"}];
    const out = withSupplied(new Set(["B"]), ps);
    const same = withSupplied(undefined, ps);
    return out.map((p) => p.name.replace("\0", "~") + ":" + p.type + ":" + (p.defaultOwner ?? "-") + ":" + (p.suppliedOf ?? "-") + ":" + String(p.optional ?? false)).join(",") + "/" + same.length;
  }
}
`, false, true)
}
