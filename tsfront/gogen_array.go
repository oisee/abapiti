package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func (l *lowerer) gogenArrayCall(n *ast.Node, name string, receiver *hir.Expr, args []*ast.Node) (*hir.Expr, bool) {
	if name == "push" && len(args) != 1 {
		return l.multiPush(n, receiver, args), true
	}
	if name == "at" {
		return l.arrayAt(n, receiver, args), true
	}
	return nil, false
}

// Snapshot the receiver and evaluate every argument before the first insertion.
// An argument can itself mutate the receiver or replace the source variable.
func (l *lowerer) multiPush(n *ast.Node, receiver *hir.Expr, args []*ast.Node) *hir.Expr {
	recv := l.tempInit(n, receiver.Type, receiver)
	type argument struct {
		value  *hir.Expr
		spread bool
	}
	var values []argument
	for _, arg := range args {
		if arg.Kind == ast.KindSpreadElement {
			saved := l.hint
			l.hint = hir.Type{}
			xs := l.iterableValues(arg.Expression())
			l.hint = saved
			if xs == nil {
				return nil
			}
			if xs.Type.Kind != hir.Array || !l.acceptsType(recv.Type.Args[0], xs.Type.Args[0]) {
				l.diagf(arg, "unsupported-call", "push spread requires a matching array iterable")
				return nil
			}
			values = append(values, argument{l.tempInit(arg, xs.Type, l.rtOp("array.slice0", xs, xs.Type)), true})
			continue
		}
		saved := l.hint
		l.hint = recv.Type.Args[0]
		value := l.expr(arg)
		l.hint = saved
		if value == nil {
			return nil
		}
		values = append(values, argument{l.tempInit(arg, recv.Type.Args[0], l.coerce(value, recv.Type.Args[0])), false})
	}
	for _, arg := range values {
		if arg.spread {
			l.serial++
			row := hir.V("$push"+itoa(l.serial), arg.value.Type.Args[0])
			l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: row.Name, Type: row.Type, X: arg.value,
				Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: l.rtOp("array.push", recv, hir.T(hir.I32), l.coerce(row, recv.Type.Args[0]))})})
		} else {
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", recv, hir.T(hir.I32), arg.value)})
		}
	}
	return l.rtOp("array.length", recv, hir.T(hir.I32))
}

// ToIntegerOrInfinity precedes relative indexing. Test bounds in Number space
// before converting to the runtime's storage index, including infinities.
func (l *lowerer) arrayAt(n *ast.Node, receiver *hir.Expr, args []*ast.Node) *hir.Expr {
	if len(args) > 1 {
		l.diagf(n, "unsupported-call", "array.at accepts at most one numeric index")
		return nil
	}
	recv := l.tempInit(n, receiver.Type, receiver)
	number, boolean := hir.T(hir.Number), hir.T(hir.Bool)
	value := hir.L(number, 0)
	if len(args) == 1 {
		value = l.expr(args[0])
	}
	if value == nil {
		return nil
	}
	if value.Type.Kind != hir.Number && value.Type.Kind != hir.I32 && value.Type.Kind != hir.I64 {
		l.diagf(n, "unsupported-call", "array.at requires a numeric index")
		return nil
	}
	index := l.tempInit(n, number, l.coerce(value, number))
	length := l.tempInit(n, number, l.coerce(l.rtOp("array.length", recv, hir.T(hir.I32)), number))
	bin := func(op string, a, b *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Op: op, Type: boolean, X: a, Y: b}
	}
	assign := func(v *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, X: index, Y: v} }
	neg := func(v *hir.Expr) *hir.Expr { return &hir.Expr{Kind: hir.Unary, Op: "-", Type: number, X: v} }
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: bin("!=", index, index), Body: hir.B(assign(hir.L(number, 0)))})
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: bin("<", index, hir.L(number, 0)),
		Body: hir.B(assign(l.rtOp("number.ceil", index, number))),
		Else: hir.B(assign(neg(l.rtOp("number.ceil", neg(index), number))))})
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: bin("<", index, hir.L(number, 0)), Body: hir.B(assign(&hir.Expr{Kind: hir.Binary, Op: "+", Type: number, X: length, Y: index}))})
	elem := recv.Type.Args[0]
	optional := elem
	if optional.Kind != hir.Optional {
		optional = hir.T(hir.Optional, elem)
	}
	result := l.tempInit(n, optional, &hir.Expr{Kind: hir.Lit, Type: optional})
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: bin("&&", bin(">=", index, hir.L(number, 0)), bin("<", index, length)),
		Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: result, Y: l.rtOp("array.get", recv, optional, l.indexValue(index))})})
	return result
}
