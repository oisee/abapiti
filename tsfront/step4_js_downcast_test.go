package tsfront

import "testing"

// Unchecked JavaScript passes a wider reference where JSDoc declares a
// subtype (frontend.mjs rowOf(INode) for an ExpressionNode parameter, an
// INode for ExpressionNode | TokenNode): a checked cast, not a refusal.
var step4JSDowncast = [2]string{`export interface INode { getChildren(): INode[]; get(): number }
export class Expr implements INode { readonly t: string; readonly kids: INode[]; constructor(t: string, kids: INode[]) { this.t = t; this.kids = kids; } getChildren(): INode[] { return this.kids; } concatTokens(): string { return this.t; } get(): number { return 1; } }
export class Tok implements INode { readonly t: string; constructor(t: string) { this.t = t; } getChildren(): INode[] { return []; } concatTokens(): string { return this.t; } get(): number { return 2; } }
`, `import {Expr, Tok} from "./types.ts";
/** @param {Expr} n @returns {string} */
function rowOf(n) { return n.concatTokens() + n.kids.length; }
/** @param {Expr | Tok} n @returns {string} */
function source(n) { return n.concatTokens(); }
/** @param {Expr | Tok | undefined} n @returns {string} */
function maybe(n) { return n === undefined ? "-" : n.concatTokens(); }
export class Probe {
  /** @returns {string} */ static run() {
    const e = new Expr("a", [new Tok("b"), new Expr("c", [])]);
    const kids = e.getChildren();
    return rowOf(kids[1]) + source(kids[0]) + source(kids[1]) + maybe(kids[0]);
  }
}
`}

func TestStep4JSImplicitDowncastNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, step4JSDowncast[0], step4JSDowncast[1], false, true)
}
