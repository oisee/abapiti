package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// A bounded lastIndexOf adapter for the primitive find predicate. Use live
// membership and strict primitive equality; holes are skipped, not read as zero.
// The optional fromIndex/coercion domain is deliberately not approximated.
func (l *lowerer) primitiveLastIndexOf(n *ast.Node, receiver *hir.Expr, args []*ast.Node) *hir.Expr {
	elem := receiver.Type.Args[0]
	base := elem
	if base.Kind == hir.Optional {
		base = base.Args[0]
	}
	if len(args) != 1 || (base.Kind != hir.String && base.Kind != hir.Number && base.Kind != hir.Bool) {
		l.diagf(n, "unsupported-call", "lastIndexOf requires one primitive search value; fromIndex and reference elements are not lowered")
		return nil
	}
	recv := l.tempInit(n, receiver.Type, receiver)
	value := l.expr(args[0])
	if value == nil {
		return nil
	}
	optional := hir.T(hir.Optional, base)
	if !value.Type.Equal(base) && !value.Type.Equal(optional) {
		l.diagf(n, "unsupported-call", "lastIndexOf search value must match its primitive element type")
		return nil
	}
	// Arguments execute before length is read, just as the JS builtin sees them.
	value = l.tempInit(n, optional, l.coerce(value, optional))
	number := hir.T(hir.Number)
	length := l.rtOp("array.length", recv, hir.T(hir.I32))
	index := l.tempInit(n, number, &hir.Expr{Kind: hir.Binary, Op: "-", Type: number, X: length, Y: hir.L(number, 1)})
	result := l.tempInit(n, number, hir.L(number, -1))
	current := l.rtOp("array.get", recv, optional, index)
	match := &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: current, Y: value}
	visit := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.rtOp("array.has", recv, hir.T(hir.Bool), index), Body: hir.B(
		&hir.Stmt{Kind: hir.If, X: match, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: result, Y: index}, &hir.Stmt{Kind: hir.Break})},
	)}
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Op: ">=", Type: hir.T(hir.Bool), X: index, Y: hir.L(number, 0)}, Body: hir.B(visit,
		&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "-", Type: number, X: index, Y: hir.L(number, 1)}},
	)})
	return result
}
