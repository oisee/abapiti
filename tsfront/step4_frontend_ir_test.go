package tsfront

import (
	"strings"
	"testing"
)

// OSG frontend-types.ts FrontendIR<T> is a recursive conditional mapped type:
// FrontendStmt = FrontendIR<Stmt> & {pos?}. Its kernel_loop member stores
// body: readonly FrontendIR<Stmt>[]: each member is a FrontendStmt member
// without the added optional pos, the same records (blockList results).
var step4FrontendIR = [2]string{`interface Meta { readonly conv?: boolean }
export type Type = Meta & ({ readonly k: "i" } | { readonly k: "c"; readonly len: number });
export type Expr = Meta & (
  | { readonly e: "var"; readonly name: string; readonly type: Type }
  | { readonly e: "int"; readonly value: number; readonly type: Type }
  | { readonly e: "nop"; readonly type: Type }
  | { readonly e: "bin"; readonly op: string; readonly l: Expr; readonly r: Expr; readonly type: Type });
export type TypedExpr = Expr & {readonly type: Type};
export interface SqlPredicate { readonly name: string; readonly type: Type; readonly kind: "eq" }
export type Stmt =
  | { readonly s: "move"; readonly target: Expr; readonly source: Expr }
  | { readonly s: "kernel_loop"; readonly fn: string; readonly binds: readonly Expr[]; readonly body: readonly Stmt[] }
  | { readonly s: "select"; readonly target: Expr; readonly preds: readonly SqlPredicate[] };
export interface FrontendSqlPredicate extends Omit<SqlPredicate, "type" | "kind"> { readonly type: Type; readonly kind?: string }
type StatementShape<T> = T extends {readonly s: "select"} ? Omit<T, "target"> & {readonly target: Expr | null} : T;
type FrontendIR<T> = T extends SqlPredicate ? FrontendSqlPredicate
  : T extends readonly (infer Item)[] ? readonly FrontendIR<Item>[]
  : T extends object ? {readonly [K in keyof StatementShape<T>]: FrontendIR<StatementShape<T>[K]>} : T;
export type FrontendStmt = FrontendIR<Stmt> & {readonly pos?: number};
export type ValueExpr = (TypedExpr | {readonly e: "nop"; readonly type: Extract<Type, {k: "c"}>}) & {readonly arith?: boolean};
`, `/** @param {string} name @returns {import("./types.ts").ValueExpr} */
function variable(name) {
  if (name === "n") return {e: "int", value: 7, type: {k: "i"}};
  return {e: "var", name, type: {k: "c", len: name.length}};
}
/** @param {import("./types.ts").Expr} x @returns {string} */
function expr(x) {
  switch (x.e) {
    case "var": return x.name + "/" + x.type.k;
    case "int": return String(x.value);
    case "nop": return "nop";
    case "bin": return "(" + expr(x.l) + x.op + expr(x.r) + ")";
  }
  throw new Error("expr");
}
/** @param {readonly import("./types.ts").FrontendStmt[]} list @returns {string} */
function show(list) {
  return list.map((s) => {
    switch (s.s) {
      case "move": return expr(s.target) + "=" + expr(s.source);
      case "kernel_loop": return s.fn + "[" + s.binds.map((b) => expr(b)).join(",") + "]{" + show(s.body) + "}@" + String(s.pos ?? -1);
      case "select": return "sel " + (s.target ? expr(s.target) : "-") + " " + s.preds.map((p) => p.name + (p.kind ?? "?")).join(",");
    }
    throw new Error("stmt");
  }).join(";");
}
/** @param {string[]} names @param {number} depth @returns {import("./types.ts").FrontendStmt[]} */
function blockList(names, depth) {
  /** @type {import("./types.ts").FrontendStmt[]} */ const out = [];
  const binds = names.map((b) => variable(b));
  const body = depth > 0 ? blockList(names.slice(1), depth - 1) : [];
  out.push({s: "kernel_loop", fn: "k" + depth, binds, body, pos: depth});
  return out;
}
export class Probe {
  /** @returns {string} */ static run() {
    return show(blockList(["ab", "n", "c"], 2));
  }
}
`}

func TestStep4FrontendIRBodyNodeOracle(t *testing.T) {
	probe := strings.Replace(step4FrontendIR[1], `@returns {import("./types.ts").ValueExpr} */
function variable`, `@returns {import("./types.ts").Expr} */
function variable`, 1)
	if probe == step4FrontendIR[1] {
		t.Fatal("probe edit")
	}
	runMjsOracleTypes(t, step4FrontendIR[0], probe, false, true)
}

// ValueExpr[] stored in binds: readonly FrontendIR<Expr>[] (Expr[]): the
// same array viewed as the wider family.
func TestStep4FrontendIRBindsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, step4FrontendIR[0], step4FrontendIR[1], false, true)
}
