package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Only the selected operand is evaluated. In the falsy arm of an optional
// object && expression, the value is undefined, even if its object arm cannot
// be admitted to the result type (for example Optional<boolean>).
func (l *lowerer) objectLogical(n *ast.Node, x *hir.Expr) (*hir.Expr, bool) {
	b := n.AsBinaryExpression()
	source := x.Type
	if source.Kind != hir.Optional || (source.Args[0].Kind != hir.ClassRef && source.Args[0].Kind != hir.InterfaceRef) {
		return nil, false
	}
	typ := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
	saved := l.pend
	l.pend = nil
	y := l.expr(b.Right)
	var yes, no *hir.Expr
	if y != nil {
		y = l.coerce(y, typ)
	}
	pending := l.pend
	l.pend = saved
	if y == nil {
		return nil, true
	}
	if len(pending) > 0 {
		y = &hir.Expr{Kind: hir.Seq, Type: y.Type, Stmt: hir.B(pending...), Y: y}
	}
	if b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken {
		yes = y
		no = l.coerce(&hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}, typ)
	} else {
		yes = l.coerce(&hir.Expr{Kind: hir.Narrow, Type: source.Args[0], X: x}, typ)
		no = y
	}
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: typ, X: &hir.Expr{Kind: hir.ToBoolean, Type: hir.T(hir.Bool), X: x}, Y: yes, Z: no}, true
}
