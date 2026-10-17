package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// setFromDynamic lowers new Set(x) for an untyped x (a JSON.parse result)
// whose Set is declared with a string or reference element type
// (`/** @type {Set<string>} */`): undefined or null give an empty Set, as
// in JavaScript; any other value must be an array of that element type,
// read through the checked live array view, which raises otherwise.
func (l *lowerer) setFromDynamic(n *ast.Node, a *hir.Expr, hint hir.Type) *hir.Expr {
	typ := hint
	if typ.Kind != hir.OrderedSet {
		before := len(l.diags)
		typ = l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
		l.diags = l.diags[:before]
	}
	if typ.Kind != hir.OrderedSet || len(typ.Args) != 1 {
		return nil
	}
	elem := typ.Args[0]
	if elem.Kind != hir.String && elem.Kind != hir.ClassRef {
		return nil
	}
	x := l.tempInit(n, a.Type, a)
	result := l.tempInit(n, typ, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ})
	nullish := l.rtOp("dynamic.isNullish", x, hir.T(hir.Bool))
	view := l.dynamicReference(x, hir.T(hir.Array, elem))
	fill := &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("set.fromArray", result, typ, view)}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: nullish}, Body: hir.B(fill)})
	l.diagf(n, "note-set-dynamic", "new Set(untyped) lowered through a checked array view")
	return result
}
