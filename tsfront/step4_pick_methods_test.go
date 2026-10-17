package tsfront

import "testing"

// Pick<C, K> over methods of a lowered class (OSG's FieldChain =
// Pick<ExpressionNode, "getChildren" | "concatTokens">) is C itself, not a
// copied shape; `self ? [n] : n.getChildren()` joins to the wider array.
var step4PickMethods = [2]string{`export interface INode { getChildren(): INode[]; get(): number }
export class Expr implements INode { readonly t: string; readonly kids: INode[]; constructor(t: string, kids: INode[]) { this.t = t; this.kids = kids; } getChildren(): INode[] { return this.kids; } concatTokens(): string { return this.t + this.kids.map((k) => k instanceof Expr ? k.concatTokens() : "?").join(""); } get(): number { return 1; } }
export class Tok implements INode { readonly t: string; constructor(t: string) { this.t = t; } getChildren(): INode[] { return []; } concatTokens(): string { return this.t; } get(): number { return 2; } }
export type FieldChain = Pick<Expr, "getChildren" | "concatTokens">;
`, `import {Expr, Tok} from "./types.ts";
/** @param {import("./types.ts").FieldChain} n @param {boolean} self @returns {string} */
function fieldChain(n, self) {
  const kids = self ? [n] : n.getChildren();
  return n.concatTokens() + ":" + kids.length + ":" + String(kids[0] === n);
}
/** @param {import("./types.ts").INode} n @returns {string} */
const viaNode = (n) => fieldChain(n, false);
/** @param {Pick<Expr, "concatTokens">} n @returns {string} */
const text = (n) => n.concatTokens();
export class Probe {
  /** @returns {string} */ static run() {
    const e = new Expr("a", [new Tok("b"), new Expr("c", [new Tok("d")])]);
    return fieldChain(e, true) + " " + fieldChain(e, false) + " " + viaNode(e.getChildren()[1]) + " " + text(e);
  }
}
`}

func TestStep4PickedMethodsAreTheClassNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, step4PickMethods[0], step4PickMethods[1], false, true)
}
