package overrides

import "github.com/oisee/abapiti/hir"

// DumpScope initializes indent to zero and only recurses with indent+1.
func IndentationRepeat(unit string) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "string.repeatIndent", Type: hir.T(hir.String), X: hir.L(hir.T(hir.String), unit), Args: []*hir.Expr{&hir.Expr{Kind: hir.Binary, Op: "*", Type: hir.T(hir.Number), X: hir.V("indent", hir.T(hir.Number)), Y: hir.L(hir.T(hir.Number), 2)}}}
}
func registryDumpIndentation() []Entry {
	return []Entry{{ID: "abaplint-dump-indentation", Key: Key{"src/lsp/dump_scope.ts", "DumpScope", "KindClassDeclaration"}, SHA256: "dfd65c4917f4d5f11eda63daabf218286d7e20ba64f04e394ed90609228f6a21", Rationale: "private dump traversal indent begins at zero and only increments by one; repeat exactly preserves existing HTML units; whole class pin guards all consumers; generalise later", Patterns: &Patterns{Expressions: map[string]func() *hir.Expr{
		`"&nbsp".repeat(indent * 2)`:  func() *hir.Expr { return IndentationRepeat("&nbsp") },
		`"&nbsp;".repeat(indent * 2)`: func() *hir.Expr { return IndentationRepeat("&nbsp;") },
	}}}}
}
