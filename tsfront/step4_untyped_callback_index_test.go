package tsfront

import "testing"

// frontend.mjs statement(): after an earlier type guard the checker narrows
// node to never, so node.getChildren()...map(...) is any and the some/every
// callback indexes are any. They are still the array indexes (numbers):
// keywords[i + j] adds, it does not concatenate.
func TestStep4UntypedCallbackIndexNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface INode { getChildren(): INode[]; text(): string }
export class Tok implements INode { readonly t: string; constructor(t: string) { this.t = t; } getChildren(): INode[] { return []; } text(): string { return this.t; } }
export class Stmt implements INode { readonly kind: string; readonly kids: INode[]; constructor(kind: string, kids: INode[]) { this.kind = kind; this.kids = kids; } getChildren(): INode[] { return this.kids; } text(): string { return ""; } }
`, `import {Tok, Stmt} from "./types.ts";
/** @param {import("./types.ts").INode} n @returns {string} */ const tokenStr = (n) => n.text();
/** @param {unknown} s @returns {string} */ const upper = (s) => String(s).toUpperCase();
/** @param {unknown} n @param {string} kind @returns {n is Stmt} */ const isStmt = (n, kind) => n instanceof Stmt && n.kind === kind;
/** @param {Stmt} node @returns {string} */
const probe = (node) => {
  if (isStmt(node, "move")) return "move";
  if (isStmt(node, "concat")) {
    const keywords = node.getChildren().filter((k) => k instanceof Tok).map((k) => upper(tokenStr(k)));
    const hasKeywords = (...words) => keywords.some((_, i) => words.every((word, j) => keywords[i + j] === word));
    const at = keywords.map((w, i) => i * 2).join(" ");
    const last = keywords.reduce(/** @param {number} acc */ (acc, w, i) => acc + i, 0);
    return [hasKeywords("A", "B"), hasKeywords("B", "C"), hasKeywords("C", "A"), hasKeywords()].map((b) => String(b)).join(",") + ":" + at + ":" + last;
  }
  return "other";
};
export class Probe {
  /** @returns {string} */ static run() { return probe(new Stmt("concat", [new Tok("a"), new Stmt("x", []), new Tok("b"), new Tok("c")])) + probe(new Stmt("move", [])); }
}
`, false, true)
}
