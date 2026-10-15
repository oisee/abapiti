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
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: v.Name, Type: row, X: src,
			Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("map.set", m, typ,
				l.coerce(&hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "f0", Type: f0, X: v}, keyT),
				l.coerce(&hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "f1", Type: f1, X: v}, valT))})})
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
	src := l.iterableValues(el.Expression())
	if src != nil && src.Type.Kind == hir.OrderedSet {
		src = l.rtOp("set.values", src, hir.T(hir.Array, src.Type.Args[0]))
	}
	if src == nil || src.Type.Kind != hir.Array || len(src.Type.Args) != 1 {
		l.diagf(el, "unsupported-array", "spread element needs an array")
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
