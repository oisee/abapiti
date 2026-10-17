package tsfront

import "testing"

// frontend.mjs cond(): after `if (isExpr(n, Compare)) return ...` the false
// branch drops ExpressionNode from n's SyntaxNode union, so the second guard
// `isExpr(n, CondSub)` narrows n to never (the remaining node classes cannot
// intersect ExpressionNode). At run time n is the ExpressionNode the guard
// accepted: n.findDirectExpression(...) dispatches on it.
func TestStep4GuardNeverNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export class Kind { readonly name: string; constructor(name: string) { this.name = name; } }
export abstract class AbstractNode { protected readonly children: AbstractNode[]; constructor(children: AbstractNode[]) { this.children = children; } getChildren(): AbstractNode[] { return this.children; } }
export class TokenNode extends AbstractNode { private readonly text: string; constructor(text: string) { super([]); this.text = text; } getStr(): string { return this.text; } }
export class ExpressionNode extends AbstractNode { private readonly kind: Kind; constructor(kind: Kind, children: AbstractNode[]) { super(children); this.kind = kind; } get(): Kind { return this.kind; }
  findDirectExpression(k: Kind): ExpressionNode | undefined { for (const c of this.children) if (c instanceof ExpressionNode && c.kind === k) return c; return undefined; } }
export class StatementNode extends AbstractNode { private readonly stmt: string; constructor(stmt: string) { super([]); this.stmt = stmt; } getStmt(): string { return this.stmt; } }
export type SyntaxNode = ExpressionNode | StatementNode | TokenNode;
`, `import {Kind, ExpressionNode, TokenNode} from "./types.ts";
const Compare = new Kind("Compare"), CondSub = new Kind("CondSub"), Cond = new Kind("Cond");
/** @param {unknown} n @param {Kind} k @returns {n is ExpressionNode} */ const isExpr = (n, k) => n instanceof ExpressionNode && n.get() === k;
/** @param {import("./types.ts").SyntaxNode | undefined} node @returns {string} */ function cond(node) {
  if (node === undefined) return "?";
  /** @param {import("./types.ts").SyntaxNode} n @returns {string} */ const one = (n) => {
    if (isExpr(n, Compare)) return "cmp";
    if (isExpr(n, CondSub)) {
      const inner = cond(n.findDirectExpression(Cond));
      return "(" + inner + ")";
    }
    return "other";
  };
  return node.getChildren().map((c) => c instanceof ExpressionNode || c instanceof TokenNode ? one(c) : "-").join(" ");
}
export class Probe {
  /** @returns {string} */ static run() {
    const leaf = new ExpressionNode(Cond, [new ExpressionNode(Compare, []), new TokenNode("AND"), new ExpressionNode(Compare, [])]);
    const root = new ExpressionNode(Cond, [new ExpressionNode(CondSub, [leaf]), new TokenNode("OR"), new ExpressionNode(CondSub, [])]);
    return cond(root);
  }
}
`, false, true)
}
