package tsfront

import "testing"

// OSG template(): `return {e: "template", parts, type: S}` where parts is a
// TemplatePart[] (a union of {text} | {value, opts}) and the result is the
// ValueExpr intersection over the Expr union. The variant's parts slot keeps
// the element union; the typed array must flow into it unchanged.
var step4TemplateParts = [2]string{`interface Meta { readonly conv?: boolean }
export type Ty = {readonly k: "s"} | {readonly k: "i"};
export interface Opts { readonly width?: number; readonly pad?: string }
export type Part = {readonly text: string} | {readonly value: Expr; readonly opts: Opts};
export type Expr = Meta & (
  | {readonly e: "lit"; readonly v: string; readonly type: Ty}
  | {readonly e: "template"; readonly parts: readonly Part[]; readonly type: Ty}
  | {readonly e: "var"; readonly name: string; readonly type: Ty});
export type TypedExpr = Expr & {readonly type: Ty};
export type ValueExpr = (TypedExpr | {readonly e: "nop"; readonly type: Ty}) & {readonly arith?: boolean};
`, `/** @type {import("./types.ts").Ty} */ const S = {k: "s"};
/** @param {string[]} words @returns {import("./types.ts").ValueExpr} */
function template(words) {
  /** @type {import("./types.ts").Part[]} */ const parts = [];
  for (const w of words) {
    if (w.startsWith("$")) parts.push({value: {e: "var", name: w.slice(1), type: S}, opts: w.length > 2 ? {width: w.length} : {}});
    else parts.push({text: w});
  }
  return withParts(parts);
}
/** @param {import("./types.ts").Part[]} parts @returns {import("./types.ts").ValueExpr} */
function withParts(parts) { return {e: "template", parts, type: S}; }
/** @param {import("./types.ts").ValueExpr} x @returns {string} */
function show(x) {
  if (x.e !== "template") return x.e;
  return x.parts.length + ":" + JSON.stringify(x.parts);
}
// Construct the family before mutating the typed source array. A copied
// array would retain the old length and miss the later element identities.
/** @returns {string} */
function liveParts() {
  /** @type {import("./types.ts").Part} */ const first = {text: "first"};
  /** @type {import("./types.ts").Part} */ const later = {value: {e: "var", name: "later", type: S}, opts: {width: 5}};
  /** @type {import("./types.ts").Part[]} */ const parts = [first];
  const value = withParts(parts);
  if (value.e !== "template") return "unexpected variant";
  const view = value.parts;
  const before = view.length;
  const pushed = parts.push(later);
  const after = view.length;
  parts.push(first);
  return (view === parts) + ":" + before + ":" + pushed + ":" + after + ":" + view.length + ":" + (view[0] === first) + ":" + (view[1] === later) + ":" + (view[2] === first) + ":" + show(value);
}
export class Probe {
  /** @returns {string} */ static run() {
    return show(template(["a", "$x", "b", "$yy"])) + " " + show(template([])) + " " + JSON.stringify(template(["q", "$z"])) + " " + liveParts();
  }
}
`}

func TestStep4TemplatePartsSlotOracle(t *testing.T) {
	runMjsOracleTypes(t, step4TemplateParts[0], step4TemplateParts[1], false, true)
}
