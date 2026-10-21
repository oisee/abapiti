package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// An asserted method contract changes no receiver or prototype. Dispatch the
// original declared method, retaining this, rather than inventing a stored
// function field for the type literal's signature.
func (l *lowerer) assertedMethodReceiverCall(n, callee *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || callee == nil || callee.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	receiver := callee.Expression()
	for receiver != nil && receiver.Kind == ast.KindParenthesizedExpression {
		receiver = receiver.Expression()
	}
	if receiver == nil || receiver.Kind != ast.KindAsExpression || receiver.Type() == nil || receiver.Type().Kind != ast.KindTypeLiteral {
		return nil, false
	}
	operand := receiver.Expression()
	typ := l.ck.GetTypeAtLocation(operand)
	if typ == nil {
		return nil, false
	}
	sym := l.ck.GetPropertyOfType(l.ck.GetNonNullableType(typ), callee.Name().Text())
	method := l.methodOf(sym)
	if method == nil || method.Static {
		return nil, false
	}
	recv := l.callReceiverExpr(operand)
	if recv == nil {
		return nil, true
	}
	// A declaration can be inherited through an interface with data slots.
	// Reuse its real method interface rather than the copied data layout.
	for _, iface := range l.out.Interfaces {
		for _, m := range iface.Methods {
			if m == method {
				target := hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}
				if !l.acceptsType(target, recv.Type) {
					recv = l.dynamicReference(l.coerce(recv, hir.T(hir.Dynamic)), target)
				}
			}
		}
	}
	recv = l.callReceiver(n, recv, method.Params)
	args, ok := l.assertedMethodArgs(n, method)
	if !ok {
		return nil, true
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Type: method.Result, Name: method.Name, X: recv, Args: args}, true
}

func (l *lowerer) assertedMethodArgs(n *ast.Node, m *hir.Method) ([]*hir.Expr, bool) {
	args, params := n.Arguments(), m.Params
	if len(args) == 1 && args[0].Kind == ast.KindSpreadElement && (len(params) == 0 || len(params) == 1 && params[0].Type.Kind == hir.Optional) {
		// A variadic asserted contract can call a real zero-argument method
		// (parse) or its one-optional-argument sibling (parseAsync). Evaluate
		// the array once; unused extra arguments do not change the receiver.
		t := l.ck.GetTypeAtLocation(args[0].Expression())
		if t != nil && l.ck.IsArrayType(t) && !t.IsTupleType() {
			x := l.expr(args[0].Expression())
			if x == nil {
				return nil, false
			}
			if x.Type.Kind == hir.Array {
				x = l.tempInit(args[0], x.Type, x)
				if len(params) == 0 {
					return nil, true
				}
				value := l.rtOp("array.get", x, hir.T(hir.Optional, x.Type.Args[0]), hir.L(hir.T(hir.I32), 0))
				return []*hir.Expr{l.coerce(value, params[0].Type)}, true
			}
		}
	}
	return l.callArgsMethod(n, args, m, 0)
}
