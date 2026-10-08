package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Search backwards in UTF-16 units, including overlapping matches. Both
// operands are evaluated once, before the loop, in source order.
func (l *lowerer) stringLastIndexOf(n *ast.Node, recv *hir.Expr, argument *ast.Node) *hir.Expr {
	str, i, b := hir.T(hir.String), hir.T(hir.Number), hir.T(hir.Bool)
	recv = l.tempInit(n, str, recv)
	needle := l.expr(argument)
	if needle == nil {
		return nil
	}
	needle = l.tempInit(n, str, needle)
	length := l.tempInit(n, i, l.rtOp("string.length", needle, i))
	pos := l.tempInit(n, i, &hir.Expr{Kind: hir.Binary, Type: i, Op: "-", X: l.rtOp("string.length", recv, i), Y: length})
	result := l.tempInit(n, i, hir.L(i, -1))
	end := &hir.Expr{Kind: hir.Binary, Type: i, Op: "+", X: pos, Y: length}
	cond := &hir.Expr{Kind: hir.Binary, Type: b, Op: "==", X: l.rtOp("string.substring", recv, str, pos, end), Y: needle}
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Type: b, Op: ">=", X: pos, Y: hir.L(i, 0)}, Body: hir.B(
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: cond, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: result, Y: pos}, &hir.Stmt{Kind: hir.Break})},
		&hir.Stmt{Kind: hir.Assign, X: pos, Y: &hir.Expr{Kind: hir.Binary, Type: i, Op: "-", X: pos, Y: hir.L(i, 1)}},
	)})
	return result
}
