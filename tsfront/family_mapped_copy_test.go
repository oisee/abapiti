package tsfront

import "testing"

// OSG's RuntimeFields<T> re-creates every member of a recursive family as a
// homomorphic mapped copy. A copy of all members is the family itself, so
// the common field `type` (Ty in every member) joins without narrowing.
func TestFamilyMappedCopyNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `type TypeMeta = {readonly qname?: string; readonly ddic?: string};
export type Ty = TypeMeta & ({readonly k: 'i'} | {readonly k: 'table'; readonly row: Ty} | {readonly k: 'struct'; readonly go: string; readonly qname?: string} | {readonly k: 'x'; readonly len: number});
type ExprMeta = {readonly conv?: boolean; readonly fromP?: Extract<Ty, {k: 'i'}>};
export type IRExpr = ExprMeta & ({readonly e: 'field'; readonly base: IRExpr; readonly name: string; readonly type: Ty} | {readonly e: 'num'; readonly value: number; readonly type: Ty} | {readonly e: 'flag'; readonly value: boolean; readonly type?: Ty} | {readonly e: 'conv'; readonly to: Ty; readonly x: IRExpr; readonly type: Ty; readonly fields?: readonly {readonly name: string; readonly len: number}[]; readonly pairs?: readonly (readonly [string, string])[]} | {readonly e: 'strlist'; readonly values: readonly string[]});
type RuntimeValue<T> = T extends IRExpr ? Expr : T extends readonly (infer V)[] ? readonly RuntimeValue<V>[] : T extends object ? RuntimeFields<T> : T;
type RuntimeFields<T> = T extends object ? {readonly [K in keyof T]: K extends 'assign' ? string : RuntimeValue<T[K]>} : T;
export type Expr = RuntimeFields<Exclude<IRExpr, {e: 'num'}>> | {readonly e: 'num'; readonly value: number; readonly type?: Ty};
`, `/** @typedef {import("./types.ts").Expr} Expr */
/** @typedef {import("./types.ts").Ty} Ty */
/** @param {Ty} t @returns {string} */
const ty = (t) => t.k === "table" ? "table<" + ty(t.row) + ">" : t.k;
/** @param {Expr} p @returns {string} */
const show = (p) => {
  switch (p.e) {
    case "field": { const t = p.type; const rowk = t.k === "table" ? t.row.k : "-"; return p.name + ":" + (p.base.type === undefined ? "-" : ty(p.base.type)) + "/" + ty(p.type) + "/" + rowk + "/" + (p.base.type?.k ?? "none"); }
    case "num": return "n" + p.value;
    case "conv": {
      const fields = p.fields ?? []; const direct = p.fields === undefined ? "" : String(p.fields.reduce((n, f) => n + f.len, 0)) + p.fields.map((f) => f.name).join("");
      const names = fields.map((f) => f.name + f.len).join("+");
      const total = fields.reduce((n, f) => n + f.len, 0);
      const found = fields.find((f) => f.len > 1);
      const last = fields.findLast((f) => f.len > 1);
      const pairs = (p.pairs ?? []).map(([t, f]) => t + "=" + f).join(",");
      /** @type {string[]} */ const idx = [];
      for (const [i, f] of fields.entries()) idx.push(i + ":" + f.name);
      return (p.to.k === "x" ? "x" + p.to.len : "c" + p.to.k) + "|" + names + "|" + total + "|" + (found ? found.name : "-") + "|" + (last ? last.name : "-") + "|" + pairs + "|" + idx.join(",") + "|" + direct;
    }
    default: return "o";
  }
};
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Expr} */ const n = {e: "num", value: 1, type: {k: "i"}};
    /** @type {Expr} */ const f = {e: "field", base: n, name: "x", type: {k: "table", row: {k: "struct", go: "S"}}};
    /** @type {Expr} */ const c = {e: "conv", to: {k: "x", len: 4}, x: n, type: {k: "i"}, fields: [{name: "a", len: 1}, {name: "b", len: 2}, {name: "c", len: 3}], pairs: [["p", "q"]]};
    return [show(n), show(f), show(c)].join(" ");
  }
}
`, false)
}
