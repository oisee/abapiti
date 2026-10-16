package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// arrayIterationCall lowers array methods outside the inlined callback set:
// findLast(cb) and entries().
func (l *lowerer) arrayIterationCall(n *ast.Node, name string, recv *hir.Expr) (*hir.Expr, bool) {
	if !l.unprunedFeatures || recv.Type.Kind != hir.Array || len(recv.Type.Args) != 1 {
		return nil, false
	}
	switch {
	case name == "findLast" && len(n.Arguments()) == 1:
		return l.findLastLoop(n, recv, n.Arguments()[0]), true
	case name == "entries" && len(n.Arguments()) == 0:
		return l.arrayEntries(n, recv), true
	}
	return nil, false
}

// findLastLoop is Array.prototype.findLast: the length is read once, indices
// run from length-1 down to 0, and the first element satisfying the predicate
// is the result (undefined when none does). Holes would pass undefined to the
// callback; reference elements narrow (an absent slot raises).
func (l *lowerer) findLastLoop(n *ast.Node, receiver *hir.Expr, cb *ast.Node) *hir.Expr {
	elem := receiver.Type.Args[0]
	if !elem.IsRef() {
		l.diagf(n, "unsupported-call", "findLast on primitive elements needs the hole-aware callback ABI")
		return nil
	}
	recv := l.tempInit(n, receiver.Type, receiver)
	body, params, ok := l.callbackBody(n, cb, elem)
	if !ok || len(params) < 1 || len(params) > 3 {
		l.diagf(n, "unsupported-call", "findLast callback needs one to three parameters")
		return nil
	}
	i, b := hir.T(hir.I32), hir.T(hir.Bool)
	bin := func(op string, x, y *hir.Expr, t hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: t, X: x, Y: y}
	}
	optional := hir.T(hir.Optional, elem)
	result := l.tempInit(n, optional, &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: optional})
	k := l.tempInit(n, i, bin("-", l.rtOp("array.length", recv, i), hir.L(i, 1), i))
	value := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: elem, X: l.rtOp("array.get", recv, optional, k)}
	l.declare(params[0], elem)
	inner := []*hir.Stmt{{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem, X: value}}
	if len(params) >= 2 {
		l.declare(params[1], hir.T(hir.Number))
		inner = append(inner, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: hir.T(hir.Number), X: l.rtOp("number.fromI32", k, hir.T(hir.Number))})
	}
	if len(params) >= 3 {
		l.declare(params[2], recv.Type)
		inner = append(inner, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[2], Type: recv.Type, X: recv})
	}
	inner = append(inner,
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.toBool(n, body), Body: hir.B(
			&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: result, Y: l.coerce(hir.V(params[0], elem), optional)},
			&hir.Stmt{Kind: hir.Break, Node: l.node(n)})},
		&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: k, Y: bin("-", k, hir.L(i, 1), i)})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: bin(">=", k, hir.L(i, 0), b), Body: hir.B(inner...)})
	return result
}

// arrayEntries snapshots arr.entries() consumed at once (for-of, spread):
// [index, value] rows, a tuple, or an array when the element type is number.
func (l *lowerer) arrayEntries(n *ast.Node, receiver *hir.Expr) *hir.Expr {
	elem := receiver.Type.Args[0]
	if !elem.IsRef() && elem.Kind != hir.Number && elem.Kind != hir.String && elem.Kind != hir.Bool {
		l.diagf(n, "unsupported-call", "entries on %s elements is not lowered", elem.Kind)
		return nil
	}
	num := hir.T(hir.Number)
	recv := l.tempInit(n, receiver.Type, receiver)
	i := hir.T(hir.I32)
	var row hir.Type
	if elem.Equal(num) {
		row = hir.T(hir.Array, num)
	} else {
		row = l.tupleShape(n, []hir.Type{num, elem})
	}
	result := l.tempVar(n, hir.T(hir.Array, row))
	l.serial++
	v := hir.V("entryValue"+itoa(l.serial), elem)
	k := l.tempInit(n, num, hir.L(num, -1))
	step := &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: k, Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: num, X: k, Y: hir.L(num, 1)}}
	index := k
	var body []*hir.Stmt
	if row.Kind == hir.Array {
		l.serial++
		pair := hir.V("entryRow"+itoa(l.serial), row)
		l.declare(pair.Name, row)
		body = []*hir.Stmt{step,
			{Kind: hir.VarDecl, Node: l.node(n), Name: pair.Name, Type: row, X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: row}},
			{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", pair, i, index)},
			{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", pair, i, v)},
			{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, i, pair)}}
	} else {
		body = []*hir.Stmt{step, {Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, i, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: row, Args: []*hir.Expr{index, v}})}}
	}
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: v.Name, Type: elem, X: recv, Body: hir.B(body...)})
	return result
}
