package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// A required reference type can still carry an absent array slot. Preserve
// that nil through the read and throw a JS TypeError only when a property is used.
// Do not turn admission failures for present values into catchable errors.
func (l *lowerer) checkedPropertyReceiver(n *ast.Node, x *hir.Expr) *hir.Expr {
	if !l.unprunedFeatures || n == nil || n.Parent == nil || n.Parent.Kind != ast.KindPropertyAccessExpression || x.Kind == hir.This {
		return x
	}
	if !arrayIndexRead(x) {
		return x
	}
	p := n.Parent.AsPropertyAccessExpression()
	if p.Expression != n || p.QuestionDotToken != nil || n.Parent.Flags&ast.NodeFlagsOptionalChain != 0 {
		return x
	}
	// Calls retain the receiver's dispatch provenance; their method boundary
	// performs its own receiver check. This guard is for data property reads.
	if call := n.Parent.Parent; call != nil && call.Kind == ast.KindCallExpression && call.Expression() == n.Parent {
		return x
	}
	switch x.Type.Kind {
	case hir.ClassRef, hir.InterfaceRef, hir.UnionRef:
	default:
		return x
	}
	value := x
	if x.Kind != hir.Local {
		value = l.tempInit(n, x.Type, x)
	}
	errType := hir.Ref(l.builtinTypeError())
	check := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: value}
	failure := &hir.Stmt{Kind: hir.Throw, Node: l.node(n), X: &hir.Expr{
		Kind: hir.New, Node: l.node(n), Type: errType,
		Args: []*hir.Expr{l.coerce(hir.L(hir.T(hir.String), "Cannot read properties of undefined"), hir.T(hir.Optional, hir.T(hir.String)))},
	}}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: check, Body: hir.B(failure)})
	return value
}
