package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Full-array fill visits every index, including holes, and returns its receiver.
// Start/end forms stay unsupported until their ToInteger/clamping ABI is proven.
func (l *lowerer) fillArray(n *ast.Node, recv *hir.Expr, args []*ast.Node) *hir.Expr {
	recv = l.tempInit(n, recv.Type, recv)
	old := l.hint
	l.hint = recv.Type.Args[0]
	value := l.expr(args[0])
	l.hint = old
	if value == nil {
		return nil
	}
	value = l.tempInit(args[0], recv.Type.Args[0], l.coerce(value, recv.Type.Args[0]))
	index := l.tempInit(n, hir.T(hir.I32), hir.L(hir.T(hir.I32), 0))
	limit := l.tempInit(n, hir.T(hir.I32), l.rtOp("array.length", recv, hir.T(hir.I32)))
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: index, Y: limit}, Body: hir.B(
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.IndexGet, Type: value.Type, X: recv, Y: index}, Y: value},
		&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}},
	)})
	return recv
}
