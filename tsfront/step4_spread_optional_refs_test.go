package tsfront

import "testing"

// OSG frontend.mjs structure(): [...st.findDirectExpressions(Source),
// ...ors.map((o) => o.findDirectExpression(Source))] spreads ExpressionNode[]
// among (ExpressionNode | undefined) slots, whose array storage differs in Go.
func TestStep4SpreadRefsIntoOptionalSlotsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export class Node { readonly name: string; readonly kids: Node[]; constructor(name: string, kids: Node[]) { this.name = name; this.kids = kids; } findAll(): Node[] { return this.kids; } findOne(n: string): Node | undefined { return this.kids.find((k) => k.name === n); } }
`, `import {Node} from "./types.ts";
export class Probe {
  /** @returns {string} */ static run() {
    const st = new Node("r", [new Node("a", []), new Node("b", [new Node("c", [])])]);
    const kids = st.findAll();
    const alts = [...kids, ...kids.map((o) => o.findOne("c"))];
    kids.push(new Node("d", []));
    return alts.map((a) => a ? a.name : "-").join(",") + ":" + alts.length + ":" + kids.length;
  }
}
`, false, true)
}
