package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// newMapFromIterable lowers new Map(iterable) for the two typed-TS sources:
// an array of [key, value] tuples (one map.set per row, later rows win, as
// in JavaScript) and another Map (a copy in insertion order). The result
// type is the checker's Map<K, V>. Anything else is left to the caller.
func (l *lowerer) newMapFromIterable(n *ast.Node) (*hir.Expr, bool) {
	ne := n.AsNewExpression()
	if ne == nil || ne.Expression == nil || ne.Expression.Kind != ast.KindIdentifier || ne.Expression.Text() != "Map" {
		return nil, false
	}
	args := n.Arguments()
	if len(args) != 1 {
		return nil, false
	}
	typ := l.hint
	if typ.Kind == hir.Optional {
		typ = typ.Args[0]
	}
	if typ.Kind != hir.OrderedMap {
		if t := l.ck.GetTypeAtLocation(n); t != nil {
			typ = l.mapCheckerType(n, t)
		}
	}
	if typ.Kind != hir.OrderedMap || len(typ.Args) != 2 {
		return nil, false
	}
	keyT, valT := typ.Args[0], typ.Args[1]
	saved := l.hint
	l.hint = hir.T(hir.Array, l.tupleShape(n, []hir.Type{keyT, valT}))
	src := l.expr(args[0])
	l.hint = saved
	if src == nil {
		return nil, true
	}
	m := l.tempInit(n, typ, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ})
	switch {
	case src.Type.Kind == hir.OrderedMap && len(src.Type.Args) == 2:
		// Walk the value snapshot and take keys from a fresh key snapshot in
		// step: a stored undefined value is copied as is (no get + presence
		// narrowing). Only the key is narrowed, so it must not be optional.
		kt, vt := src.Type.Args[0], src.Type.Args[1]
		if kt.Kind == hir.Optional {
			l.diagf(n, "unsupported-new", "new Map(map) with possibly undefined keys")
			return nil, true
		}
		keys := l.tempInit(n, hir.T(hir.Array, kt), l.rtOp("map.keys", src, hir.T(hir.Array, kt)))
		l.serial++
		value := hir.V("v"+itoa(l.serial), vt)
		key := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: kt, X: l.rtOp("array.shift", keys, hir.T(hir.Optional, kt))}
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: value.Name, Type: vt, X: l.rtOp("map.values", src, hir.T(hir.Array, vt)),
			Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("map.set", m, typ, l.coerce(key, keyT), l.coerce(value, valT))})})
		return m, true
	case src.Type.Kind == hir.Array && len(src.Type.Args) == 1 && src.Type.Args[0].Kind == hir.ClassRef:
		row := src.Type.Args[0]
		var f0, f1 hir.Type
		for _, c := range l.out.Classes {
			// Only tupleShape's own [key, value] classes qualify (fail closed).
			if c.Name != row.Name || !strings.HasPrefix(c.Name, "tuple.") || len(c.Fields) != 2 {
				continue
			}
			for _, f := range c.Fields {
				switch f.Name {
				case "f0":
					f0 = f.Type
				case "f1":
					f1 = f.Type
				}
			}
		}
		if f0.Kind == "" || f1.Kind == "" {
			l.diagf(n, "unsupported-new", "new Map needs [key, value] rows")
			return nil, true
		}
		l.serial++
		v := hir.V("r"+itoa(l.serial), row)
		// Coercing a row field may introduce temporaries; they read the row
		// variable, so they belong inside the loop body.
		saved := l.pend
		l.pend = nil
		set := &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("map.set", m, typ,
			l.coerce(&hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "f0", Type: f0, X: v}, keyT),
			l.coerce(&hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "f1", Type: f1, X: v}, valT))}
		body := append(l.pend, set)
		l.pend = saved
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: v.Name, Type: row, X: src, Body: hir.B(body...)})
		return m, true
	}
	l.diagf(n, "unsupported-new", "new Map needs an array of [key, value] rows or a Map")
	return nil, true
}

// iterableValues lowers an expression that is consumed at once as an
// iterable (a spread element, new Set's argument). A Map's or Set's
// keys()/values() iterator is then equivalent to a snapshot array of the
// same elements in insertion order; nothing can mutate the collection
// between creating and draining it here. Other expressions lower as usual.
func (l *lowerer) iterableValues(n *ast.Node) *hir.Expr {
	for n != nil && n.Kind == ast.KindParenthesizedExpression {
		n = n.Expression()
	}
	if x, ok := l.emptyCollectionFallback(n); ok {
		return x
	}
	if x, ok := l.optionalIterable(n); ok {
		return x
	}
	if n != nil && n.Kind == ast.KindCallExpression && len(n.Arguments()) == 0 {
		callee := n.Expression()
		if callee != nil && callee.Kind == ast.KindPropertyAccessExpression {
			name := callee.Name().Text()
			if name == "keys" || name == "values" {
				// Decide from the checker type before lowering anything, so the
				// receiver is lowered (and evaluated) exactly once either way.
				kind := hir.Void
				if t := l.ck.GetTypeAtLocation(callee.Expression()); t != nil {
					before := len(l.diags)
					kind = l.mapCheckerType(callee.Expression(), t).Kind
					l.diags = l.diags[:before]
				}
				if kind != hir.OrderedMap && kind != hir.OrderedSet {
					return l.expr(n)
				}
				recv := l.expr(callee.Expression())
				if recv == nil {
					return nil
				}
				if l.unprunedFeatures && recv.Type.Kind == hir.Dynamic {
					recv = l.checkedContainerReceiver(callee.Expression(), recv)
				}
				switch {
				case recv.Type.Kind == hir.OrderedMap && len(recv.Type.Args) == 2:
					elem := recv.Type.Args[0]
					op := "map.keys"
					if name == "values" {
						elem, op = recv.Type.Args[1], "map.values"
					}
					return l.rtOp(op, recv, hir.T(hir.Array, elem))
				case recv.Type.Kind == hir.OrderedSet && len(recv.Type.Args) == 1:
					return l.rtOp("set.values", recv, hir.T(hir.Array, recv.Type.Args[0]))
				}
				return l.expr(n)
			}
		}
	}
	return l.expr(n)
}

// spreadInto adds each element of a spread inside new Set([...]) or
// new Array([...]) to the collection under construction, in order.
func (l *lowerer) spreadInto(el *ast.Node, kind string, recv *hir.Expr, elemType hir.Type) *hir.Stmt {
	src := l.spreadValues(el, elemType, false)
	if src == nil {
		return nil
	}
	l.serial++
	v := hir.V("e"+itoa(l.serial), src.Type.Args[0])
	op, res := "set.add", recv.Type
	if kind == "Array" {
		op, res = "array.push", hir.T(hir.I32)
	}
	return &hir.Stmt{Kind: hir.ForEach, Node: l.node(el), Name: v.Name, Type: v.Type, X: src,
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp(op, recv, res, l.coerce(v, elemType))})}
}

// optionalIterable handles "x?.keys() ?? fallback" (also values()) for an
// optional Map/Set x consumed at once: the fallback when x is undefined,
// otherwise the snapshot. x is evaluated once; the fallback only when used.
func (l *lowerer) optionalIterable(n *ast.Node) (*hir.Expr, bool) {
	if n == nil || n.Kind != ast.KindBinaryExpression {
		return nil, false
	}
	b := n.AsBinaryExpression()
	if b.OperatorToken.Kind != ast.KindQuestionQuestionToken {
		return nil, false
	}
	left := b.Left
	for left != nil && left.Kind == ast.KindParenthesizedExpression {
		left = left.Expression()
	}
	if left == nil || left.Kind != ast.KindCallExpression || len(left.Arguments()) != 0 {
		return nil, false
	}
	callee := left.Expression()
	if callee == nil || callee.Kind != ast.KindPropertyAccessExpression || callee.AsPropertyAccessExpression().QuestionDotToken == nil {
		return nil, false
	}
	name := callee.Name().Text()
	if name != "keys" && name != "values" {
		return nil, false
	}
	t := l.ck.GetTypeAtLocation(callee.Expression())
	if t == nil {
		return nil, false
	}
	before := len(l.diags)
	recvType := l.mapCheckerType(callee.Expression(), t)
	l.diags = l.diags[:before]
	if recvType.Kind != hir.Optional {
		return nil, false
	}
	base := recvType.Args[0]
	var elem hir.Type
	op := ""
	switch {
	case base.Kind == hir.OrderedMap && len(base.Args) == 2:
		elem, op = base.Args[0], "map.keys"
		if name == "values" {
			elem, op = base.Args[1], "map.values"
		}
	case base.Kind == hir.OrderedSet && len(base.Args) == 1:
		elem, op = base.Args[0], "set.values"
	default:
		return nil, false
	}
	arr := hir.T(hir.Array, elem)
	recv := l.expr(callee.Expression())
	if recv == nil {
		return nil, true
	}
	if l.unprunedFeatures && recv.Type.Kind == hir.Dynamic {
		// A member of a receiver lowered as a tagged value: checked view.
		recv = l.coerce(recv, recvType)
	}
	if recv.Type.Kind != hir.Optional {
		return l.rtOp(op, recv, arr), true
	}
	saved := l.tempInit(n, recv.Type, recv)
	// The fallback is lowered in its own prelude scope so it runs only when used.
	pend := l.pend
	l.pend = nil
	hint := l.hint
	l.hint = arr
	fallback := l.expr(b.Right)
	l.hint = hint
	inner := l.pend
	l.pend = pend
	if fallback == nil {
		return nil, true
	}
	if fallback.Type.Kind == hir.OrderedSet {
		fallback = l.rtOp("set.values", fallback, hir.T(hir.Array, fallback.Type.Args[0]))
	}
	if fallback.Type.Kind != hir.Array {
		l.diagf(b.Right, "unsupported-array", "?? fallback of an iterable needs an array")
		return nil, true
	}
	result := l.tempInit(n, arr, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: arr})
	present := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: base, X: saved}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: saved},
		Body: hir.B(append(inner, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: result, Y: l.coerce(fallback, arr)})...),
		Else: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: result, Y: l.rtOp(op, present, arr)})})
	return result, true
}

// An immediately drained Map/Set ?? [] joins iterator elements, rather than
// pretending that an array and a Map have the same storage. Snapshot only
// the selected present branch; an absent collection yields a fresh empty array.
func (l *lowerer) emptyCollectionFallback(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || n == nil || n.Kind != ast.KindBinaryExpression {
		return nil, false
	}
	b := n.AsBinaryExpression()
	if b.OperatorToken.Kind != ast.KindQuestionQuestionToken {
		return nil, false
	}
	right := b.Right
	for right.Kind == ast.KindParenthesizedExpression {
		right = right.Expression()
	}
	if right.Kind != ast.KindArrayLiteralExpression || len(right.AsArrayLiteralExpression().Elements.Nodes) != 0 {
		return nil, false
	}
	typ := l.conditionalCheckerType(b.Left, l.ck.GetTypeAtLocation(b.Left))
	if typ.Kind == hir.Optional {
		typ = typ.Args[0]
	}
	if typ.Kind != hir.OrderedMap && typ.Kind != hir.OrderedSet {
		return nil, false
	}
	recv := l.expr(b.Left)
	if recv == nil {
		return nil, true
	}
	if recv.Type.Kind == hir.Dynamic || recv.Type.Kind == hir.Optional && recv.Type.Args[0].Kind == hir.Dynamic {
		// A collection read through an erased program union retains its
		// checker-proven collection view, including optional-chain absence.
		target := typ
		if recv.Type.Kind == hir.Optional || typeMayBeUndefined(l.ck.GetTypeAtLocation(b.Left)) {
			target = hir.T(hir.Optional, typ)
		}
		recv = l.coerce(recv, target)
	}
	recv = l.tempInit(b.Left, recv.Type, recv)
	present := recv
	optional := recv.Type.Kind == hir.Optional
	if optional {
		present = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[0], X: recv}
	}
	saved := l.pend
	l.pend = nil
	var snapshot *hir.Expr
	if present.Type.Kind == hir.OrderedMap {
		elem := hir.Type{}
		if l.hint.Kind == hir.Array {
			elem = l.hint.Args[0]
		}
		snapshot = l.mapEntryRows(n, present, elem)
	} else if present.Type.Kind == hir.OrderedSet {
		snapshot = l.rtOp("set.values", present, hir.T(hir.Array, present.Type.Args[0]))
	}
	inner := l.pend
	l.pend = saved
	if snapshot == nil {
		l.diagf(n, "unsupported-array", "empty iterable fallback requires a typed collection")
		return nil, true
	}
	if len(inner) > 0 {
		snapshot = &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: snapshot.Type, Stmt: hir.B(inner...), Y: snapshot}
	}
	if !optional {
		return snapshot, true
	}
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: snapshot.Type, X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: recv}, Y: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: snapshot.Type}, Z: snapshot}, true
}
