package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Includes observes identity, so an erased assertion need not construct the
// asserted nominal class view. Preserve the original value and compare tags.
func (l *lowerer) referenceAssertionIncludes(n *ast.Node, recv *hir.Expr, args []*ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || len(args) != 1 || !recv.Type.Args[0].IsRef() || recv.Type.Args[0].Kind == hir.Dynamic {
		return nil, false
	}
	arg := ast.SkipParentheses(args[0])
	if arg.Kind != ast.KindAsExpression && arg.Kind != ast.KindTypeAssertionExpression {
		return nil, false
	}
	for arg.Kind == ast.KindAsExpression || arg.Kind == ast.KindTypeAssertionExpression {
		arg = ast.SkipParentheses(arg.Expression())
	}
	recv = l.tempInit(n, recv.Type, recv)
	value := l.expr(arg)
	if value == nil {
		return nil, true
	}
	dyn, boolean := hir.T(hir.Dynamic), hir.T(hir.Bool)
	value = l.tempInit(n, dyn, l.coerce(value, dyn))
	result := l.tempInit(n, boolean, hir.L(boolean, false))
	l.serial++
	element := hir.V("includesValue"+itoa(l.serial), recv.Type.Args[0])
	saved := l.pend
	l.pend = nil
	equal := l.rtOp("dynamic.strictEquals", l.coerce(element, dyn), boolean, value)
	body := hir.B(append(l.pend, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: equal, Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: result, Y: hir.L(boolean, true)}, &hir.Stmt{Kind: hir.Break, Node: l.node(n)})})...)
	l.pend = saved
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: element.Name, Type: element.Type, X: recv, Body: body})
	return result, true
}
