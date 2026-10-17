package tsfront

import "testing"

// OSG G batch 10 (eb38612e) typed frontend-static.mjs staticSlot with
// `@returns {StaticSlotExpr}`, a named interface structurally equal to the
// static member of the recursive Expr family. The family must stay a class
// family: its fields and the twin's values are read as before.
func TestStep4NamedVariantTwinNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `type Meta = {readonly conv?: boolean};
export type Type = {readonly k: "i"} | {readonly k: "c"; readonly len: number};
export type Expr = Meta & (
  | {readonly e: "var"; readonly name: string; readonly type: Type}
  | {readonly e: "static"; readonly owner: string; readonly name: string; readonly type: Type; readonly go: string}
  | {readonly e: "field"; readonly base: Expr; readonly name: string; readonly type: Type});
export type ValueExpr = Expr & {readonly arith?: boolean};
export interface StaticSlotExpr { readonly e: "static"; readonly owner: string; readonly name: string; readonly type: Type; readonly go: string }
`, `/** @param {string} owner @param {string} name @param {import("./types.ts").Type} type @returns {import("./types.ts").StaticSlotExpr} */
const staticSlot = (owner, name, type) => ({e: "static", owner, name, type, go: owner + "_" + name});
/** @param {import("./types.ts").Expr} x @returns {string} */
function show(x) {
  if (x.e === "field") return show(x.base) + "-" + x.name;
  if (x.e === "static") return x.go + ":" + x.type.k;
  return x.name + (x.conv ? "!" : "");
}
/** @param {string} n @returns {import("./types.ts").ValueExpr | undefined} */
function find(n) {
  if (n === "s") return staticSlot("ZCL", "A", {k: "c", len: 3});
  if (n === "v") return {e: "var", name: "lv", type: {k: "i"}, conv: true};
  return undefined;
}
export class Probe {
  /** @returns {string} */ static run() {
    const s = find("s"), v = find("v");
    /** @type {import("./types.ts").Expr[]} */ const xs = [];
    if (s) xs.push(s, {e: "field", base: s, name: "F", type: {k: "i"}});
    if (v) xs.push(v);
    return xs.map((x) => show(x) + "/" + x.type.k).join(",");
  }
}
`, false, true)
}
