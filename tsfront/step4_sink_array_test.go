package tsfront

import "testing"

// frontend.mjs structure(): `const branches = [{cond, body}]; ...
// branches.push({cond, body}); return {s: "if", branches, else}` and
// types-signatures: `const secondary = keys.map((k) => ({name, ...}));
// return {k: "table", row, secondary}`. The local array of fresh literals
// takes the element type of the one typed slot it is stored into.
func TestStep4SinkArrayNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Branch { readonly cond: string; readonly body: readonly string[] }
export interface SecondaryKey { readonly name: string; readonly comps: readonly string[]; readonly sorted?: boolean; readonly unique?: boolean }
export type Stmt = { readonly s: "if"; readonly branches: readonly Branch[]; readonly else: readonly string[] | null } | { readonly s: "nop" };
export interface Key { name: string; type: string; unique: boolean; fields: string[] }
export type Type = { readonly k: "table"; readonly row: Type; readonly secondary?: readonly SecondaryKey[] } | { readonly k: "i" };
`, `/** @param {string[]} conds @returns {import("./types.ts").Stmt} */
function ifStmt(conds) {
  if (conds.length === 0) return {s: "nop"};
  const branches = [{cond: conds[0], body: [conds[0] + "!"]}];
  for (const c of conds.slice(1)) branches.push({cond: c, body: []});
  return {s: "if", branches, else: conds.length > 2 ? ["e"] : null};
}
/** @param {import("./types.ts").Key[]} keys @returns {import("./types.ts").Type} */
function table(keys) {
  const secondary = keys.map((k) => ({name: k.name.toUpperCase(), sorted: k.type === "SORTED", unique: k.unique, comps: k.fields}));
  return secondary.length ? {k: "table", row: {k: "i"}, secondary} : {k: "table", row: {k: "i"}};
}
export class Probe {
  /** @returns {string} */ static run() {
    const out = [];
    for (const conds of [[], ["a"], ["a", "b", "c"]]) {
      const s = ifStmt(conds);
      out.push(s.s === "if" ? s.branches.map((b) => b.cond + b.body.join("")).join("|") + String(s.else) : s.s);
    }
    /** @type {import("./types.ts").Key[][]} */ const all = [[], [{name: "k", type: "SORTED", unique: true, fields: ["x", "y"]}]];
    for (const keys of all) {
      const t = table(keys);
      out.push(t.k === "table" ? (t.secondary ?? []).map((k) => k.name + k.comps.join("") + String(k.sorted) + String(k.unique)).join("|") : t.k);
    }
    return out.join(",");
  }
}
`, false, true)
}
