package tsfront

import "testing"

// A concise arrow body takes the declared result as its contextual type,
// as a return statement does: ({k: "c", len}) builds the c variant of
// gogen's IR Type family, not a structurally equal sibling (x).
func TestConciseArrowContextualReturnNodeOracle(t *testing.T) {
	types := `interface TypeMetadata { readonly qname?: string; readonly ddic?: string }
export type Type = TypeMetadata & (
  | { readonly k: 'i' }
  | { readonly k: 'p'; readonly len: number; readonly dec: number; readonly calc?: boolean }
  | { readonly k: 'string' }
  | { readonly k: 'c'; readonly len: number }
  | { readonly k: 'x'; readonly len: number }
  | { readonly k: 'ref'; readonly name: string; readonly intf?: boolean }
  | { readonly k: 'table'; readonly row: Type });
`
	runMjsOracleTypes(t, types, `/** @type {Extract<import("./types.ts").Type, {k: "string"}>} */ const S = {k: "string"};
/** @param {number} len @returns {Extract<import("./types.ts").Type, {k: "c"}>} */ const C = (len) => ({k: "c", len});
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Type} */ const t = C(3);
    return S.k + C(5).k + C(5).len + t.k;
  }
}
`, false)
}
