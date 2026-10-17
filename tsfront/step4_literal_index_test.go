package tsfront

import "testing"

// frontend-types-signatures.mjs namedType: `const builtin = {I, F, STRING: S,
// D: {k: "d", len: 8}, ...}[t]; if (builtin) return builtin;` indexes a known
// object literal by any string. Own keys give their value, every other key
// gives undefined (no Object.prototype key is reached here).
func TestStep4LiteralIndexNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `interface TypeMetadata { readonly qname?: string; readonly ddic?: string }
export type Type = TypeMetadata & (
  | { readonly k: 'i' }
  | { readonly k: 'string' }
  | { readonly k: 'p'; readonly len: number; readonly dec: number; readonly calc?: boolean }
  | { readonly k: 'c'; readonly len: number }
  | { readonly k: 'd'; readonly len?: number }
  | { readonly k: 'ref'; readonly name: string; readonly intf?: boolean }
  | { readonly k: 'table'; readonly row: Type });
`, `/** @type {Extract<import("./types.ts").Type, {k: "i"}>} */ const I = {k: "i"};
/** @type {Extract<import("./types.ts").Type, {k: "string"}>} */ const S = {k: "string"};
/** @param {number} len @returns {Extract<import("./types.ts").Type, {k: "c"}>} */ const C = (len) => ({k: "c", len});
/** @param {import("./types.ts").Type} t @returns {string} */
const show = (t) => t.k === "c" || t.k === "d" ? t.k + (t.len ?? "-") : t.k === "p" ? "p" + t.len + "." + t.dec : t.k === "ref" ? "ref:" + t.name + ":" + t.intf : t.k === "table" ? "table of " + show(t.row) : t.k;
/** @param {string} text @returns {import("./types.ts").Type} */
function namedType(text) {
  const t = text.toUpperCase();
  const builtin = {
    I, STRING: S, D: {k: "d", len: 8},
    OBJECT: {k: "ref", name: "OBJECT", intf: true},
    P: {k: "p", len: 8, dec: 0}, C: C(1),
  }[t];
  if (builtin) return builtin;
  return t.startsWith("TT_") ? {k: "table", row: namedType(t.slice(3))} : {k: "ref", name: t};
}
export class Probe {
  /** @returns {string} */ static run() {
    return ["i", "string", "d", "object", "p", "c", "zcl_x", "", "tt_d", "c"].map((s) => show(namedType(s))).join(",") + ":" + String(namedType("i") === I);
  }
}
`, false, true)
}
