package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func (l *lowerer) gogenArrayCall(n *ast.Node, name string, receiver *hir.Expr, args []*ast.Node) (*hir.Expr, bool) {
	// A lone spread keeps the established loop (abaplint's closure relies on
	// it); mixed and multi-argument pushes take the hole-aware path below.
	if name == "push" && len(args) == 1 && args[0].Kind == ast.KindSpreadElement && receiver.Type.Args[0].Kind != hir.Optional {
		if !receiver.Type.Args[0].IsRef() && l.deletedSpreadSource(args[0].Expression()) {
			l.diagf(args[0], "unsupported-call", "push spread element storage cannot represent undefined from array holes")
			return nil, true
		}
		l.diagf(n, "note-spread-push", "push(...x) inlined into a loop")
		return l.spreadPush(n, receiver, args[0]), true
	}
	if name == "push" && (len(args) != 1 || args[0].Kind == ast.KindSpreadElement) {
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
			if xs.Type.Kind == hir.OrderedSet {
				xs = l.rtOp("set.values", xs, hir.T(hir.Array, xs.Type.Args[0]))
			}
			if xs.Type.Kind != hir.Array || !l.acceptsType(recv.Type.Args[0], xs.Type.Args[0]) {
				l.diagf(arg, "unsupported-call", "push spread requires a matching array iterable")
				return nil
			}
			if recv.Type.Args[0].Kind != hir.Optional && !recv.Type.Args[0].IsRef() {
				if l.deletedSpreadSource(arg.Expression()) {
					l.diagf(arg, "unsupported-call", "push spread element storage cannot represent undefined from array holes")
					return nil
				}
				xs = l.checkedDenseSpread(arg, xs)
			} else {
				xs = l.coerce(l.spreadSnapshot(arg, xs), recv.Type)
			}
			values = append(values, argument{l.tempInit(arg, xs.Type, xs), true})
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

// spreadSnapshot materializes iterator values: holes become present undefined
// elements. Array literal spreads and push spreads share this boundary.
func (l *lowerer) spreadSnapshot(n *ast.Node, xs *hir.Expr) *hir.Expr {
	return l.spreadSnapshotAs(n, xs, xs.Type)
}

// spreadSnapshotAs copies a spread source into a fresh array of typ. A
// reference source spread among optional slots ([...nodes, ...maybeNodes])
// has its own element storage: each element is converted as it is pushed.
func (l *lowerer) spreadSnapshotAs(n *ast.Node, xs *hir.Expr, into hir.Type) *hir.Expr {
	typ := xs.Type
	if typ.Args[0].IsRef() {
		source := l.tempInit(n, typ, xs)
		result := l.tempInit(n, into, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: into})
		index := l.tempInit(n, hir.T(hir.I32), hir.L(hir.T(hir.I32), 0))
		limit := l.tempInit(n, hir.T(hir.I32), l.rtOp("array.length", source, hir.T(hir.I32)))
		value := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: typ.Args[0], X: l.rtOp("array.get", source, hir.T(hir.Optional, typ.Args[0]), index)}
		if !into.Equal(typ) {
			value = l.coerce(value, into.Args[0])
		}
		l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: index, Y: limit},
			Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, hir.T(hir.I32), value)},
				&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}})})
		return result
	}
	if typ.Args[0].Kind != hir.Optional && !typ.Args[0].IsRef() {
		typ = hir.T(hir.Array, hir.T(hir.Optional, typ.Args[0]))
	}
	x := l.rtOp("array.spread0", xs, typ)
	x.Node = l.node(n)
	return x
}

// Statically reject a visibly deleted source; unknown producers are checked at
// runtime before any insertion. A density assumption must never fabricate zero.
func (l *lowerer) deletedSpreadSource(n *ast.Node) bool {
	if n.Kind != ast.KindIdentifier {
		return false
	}
	symbol := l.resolve(n)
	if symbol == nil {
		return false
	}
	deleted := false
	walk(l.file.AsNode(), func(x *ast.Node) {
		if x.Kind != ast.KindDeleteExpression {
			return
		}
		operand := x.AsDeleteExpression().Expression
		if operand.Kind == ast.KindElementAccessExpression && l.resolve(operand.Expression()) == symbol {
			deleted = true
		}
	})
	return deleted
}

func (l *lowerer) checkedDenseSpread(n *ast.Node, xs *hir.Expr) *hir.Expr {
	snapshot := l.tempInit(n, xs.Type, l.rtOp("array.slice0", xs, xs.Type))
	index := l.tempInit(n, hir.T(hir.I32), hir.L(hir.T(hir.I32), 0))
	limit := l.tempInit(n, hir.T(hir.I32), l.rtOp("array.length", snapshot, hir.T(hir.I32)))
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n),
		X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: index, Y: limit},
		Body: hir.B(
			&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: l.rtOp("array.has", snapshot, hir.T(hir.Bool), index)},
				Body: hir.B(&hir.Stmt{Kind: hir.Trap, Node: l.node(n), Name: l.node(n).Source + ": push spread element storage cannot represent undefined from array holes"})},
			&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}})})
	return snapshot
}
