package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// concat evaluates the receiver and every argument before copying any array.
// Array operands are spread one level; scalar operands contribute one item.
func (l *lowerer) arrayConcat(n *ast.Node, recv *hir.Expr, args []*ast.Node) *hir.Expr {
	// Keep the established single-array lowering and default output stable.
	if len(args) == 1 && args[0].Kind != ast.KindSpreadElement {
		value := l.presentValue(args[0], l.expr(args[0]), hir.Type{})
		if value == nil {
			return nil
		}
		if value.Type.Kind == hir.Array {
			return l.rtOp("array.concat", recv, recv.Type, value)
		}
		recv = l.tempInit(n, recv.Type, recv)
		value = l.tempInit(args[0], value.Type, value)
		return l.concatValues(n, recv, []*hir.Expr{value})
	}
	recv = l.tempInit(n, recv.Type, recv)
	values := make([]*hir.Expr, 0, len(args))
	for _, arg := range args {
		if arg.Kind == ast.KindSpreadElement {
			l.diagf(arg, "unsupported-call", "concat spread arguments require runtime argument expansion")
			return nil
		}
		value := l.presentValue(arg, l.expr(arg), hir.Type{})
		if value == nil {
			return nil
		}
		values = append(values, l.tempInit(arg, value.Type, value))
	}
	return l.concatValues(n, recv, values)
}

func (l *lowerer) concatValues(n *ast.Node, recv *hir.Expr, values []*hir.Expr) *hir.Expr {
	result := l.rtOp("array.slice0", recv, recv.Type)
	for _, value := range values {
		if value.Type.Kind == hir.Dynamic || value.Type.Kind == hir.Optional && value.Type.Args[0].Kind == hir.Dynamic {
			l.diagf(n, "unsupported-call", "concat tagged arguments require runtime array spreading")
			return nil
		}
		if value.Type.Kind == hir.Array {
			if !l.acceptsType(recv.Type, value.Type) {
				l.diagf(n, "unsupported-call", "concat array element conversion is not lowered")
				return nil
			}
			result = l.rtOp("array.concat", result, recv.Type, value)
		} else {
			result = l.tempInit(n, recv.Type, result)
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, hir.T(hir.I32), l.coerce(value, recv.Type.Args[0]))})
		}
	}
	return result
}
