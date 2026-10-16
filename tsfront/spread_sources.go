package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// spreadValues lowers the operand of a spread element to an array snapshot
// in JavaScript iteration order: arrays as they are, Set values, Map entries
// ([key, value] rows of the context's element type), strings by code point
// and s.matchAll(re) as its match arrays. Generators are the caller's.
// A refused operand that already reported its own blocking diagnostic does
// not add a second "needs an array" cascade.
func (l *lowerer) spreadValues(el *ast.Node, elem hir.Type, generators bool) *hir.Expr {
	operand := el.Expression()
	for operand != nil && operand.Kind == ast.KindParenthesizedExpression {
		operand = operand.Expression()
	}
	before := len(l.diags)
	var sv *hir.Expr
	if l.unprunedFeatures && isMatchAllCall(operand) {
		sv = l.matchAllValues(operand)
	} else {
		sv = l.iterableValues(operand)
	}
	if sv != nil && generators && l.generatorABIFor(sv.Type) != nil {
		sv = l.drainGenerator(sv)
	}
	if sv != nil && l.unprunedFeatures && sv.Type.Kind == hir.Optional && len(sv.Type.Args) == 1 {
		// TypeScript typed the operand present; spreading undefined throws.
		switch sv.Type.Args[0].Kind {
		case hir.Array, hir.OrderedSet, hir.OrderedMap, hir.String:
			sv = &hir.Expr{Kind: hir.Narrow, Node: l.node(el), Type: sv.Type.Args[0], X: sv}
		}
	}
	if sv != nil && sv.Type.Kind == hir.OrderedSet {
		sv = l.rtOp("set.values", sv, hir.T(hir.Array, sv.Type.Args[0]))
	}
	if sv != nil && l.unprunedFeatures && sv.Type.Kind == hir.OrderedMap && len(sv.Type.Args) == 2 {
		sv = l.mapEntryRows(el, sv, elem)
		if sv == nil {
			return nil
		}
	}
	if sv != nil && l.unprunedFeatures && sv.Type.Kind == hir.String {
		sv = l.codePointValues(el, sv)
	}
	if sv != nil && l.unprunedFeatures && sv.Type.Kind == hir.Dynamic {
		// A tagged iterable (Set | array, or a string): the runtime snapshot
		// raises for a non-iterable, as JavaScript throws a TypeError.
		sv = l.rtOp("dynamic.spread", sv, hir.T(hir.Array, hir.T(hir.Dynamic)))
		if elem.Kind != "" && elem.Kind != hir.Void && elem.Kind != hir.Dynamic {
			sv = l.convertedElements(el, sv, elem)
		}
	}
	if sv == nil || sv.Type.Kind != hir.Array || len(sv.Type.Args) != 1 {
		if sv != nil || !hasBlocking(l.diags[before:]) {
			l.diagf(el, "unsupported-array", "spread element needs an array")
		}
		return nil
	}
	return sv
}

// mapEntryRows snapshots a Map's entries in insertion order as rows of the
// spread context's element type: a [K, V] tuple, or an array when K and V
// lower to the same type.
func (l *lowerer) mapEntryRows(el *ast.Node, m *hir.Expr, elem hir.Type) *hir.Expr {
	key, value := m.Type.Args[0], m.Type.Args[1]
	if elem.Kind == "" || elem.Kind == hir.Void {
		elem = l.tupleShape(el, []hir.Type{key, value})
	}
	m = l.tempInit(el, m.Type, m)
	l.serial++
	k := hir.V("entryKey"+itoa(l.serial), key)
	v := l.rtOp("map.get", m, hir.T(hir.Optional, value), k)
	if value.Kind != hir.Optional {
		v = &hir.Expr{Kind: hir.Narrow, Node: l.node(el), Type: value, X: v}
	} else {
		l.diagf(el, "unsupported-array", "Map spread with optional values needs a presence-preserving nested optional ABI")
		return nil
	}
	var row *hir.Expr
	var rowPrelude []*hir.Stmt
	switch {
	case elem.Kind == hir.Array && len(elem.Args) == 1:
		// [K, V] rows whose element type is one array type (K and V lower to
		// the same type, or the context widened both to it).
		pending := len(l.pend)
		ck, cv := l.coerce(k, elem.Args[0]), l.coerce(v, elem.Args[0])
		if len(l.pend) != pending || !ck.Type.Equal(elem.Args[0]) || !cv.Type.Equal(elem.Args[0]) {
			l.diagf(el, "unsupported-array", "Map spread entry types do not match the [key, value] element context")
			return nil
		}
		l.serial++
		pair := hir.V("entryRow"+itoa(l.serial), elem)
		l.declare(pair.Name, elem)
		i := hir.T(hir.I32)
		rowPrelude = []*hir.Stmt{
			{Kind: hir.VarDecl, Node: l.node(el), Name: pair.Name, Type: elem, X: &hir.Expr{Kind: hir.New, Node: l.node(el), Type: elem}},
			{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp("array.push", pair, i, ck)},
			{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp("array.push", pair, i, cv)},
		}
		row = pair
	case elem.Kind == hir.ClassRef && strings.HasPrefix(elem.Name, "tuple."):
		c := shapeClassOf(l.out.Classes, elem)
		if c == nil || c.Ctor == nil || len(c.Ctor.Params) != 2 {
			l.diagf(el, "unsupported-array", "Map spread needs a [key, value] element context")
			return nil
		}
		pending := len(l.pend)
		ck, cv := l.coerce(k, c.Ctor.Params[0].Type), l.coerce(v, c.Ctor.Params[1].Type)
		if len(l.pend) != pending || !ck.Type.Equal(c.Ctor.Params[0].Type) || !cv.Type.Equal(c.Ctor.Params[1].Type) {
			l.diagf(el, "unsupported-array", "Map spread entry types do not match the [key, value] element context")
			return nil
		}
		row = &hir.Expr{Kind: hir.New, Node: l.node(el), Type: elem, Args: []*hir.Expr{ck, cv}}
	default:
		l.diagf(el, "unsupported-array", "Map spread needs a [key, value] element context")
		return nil
	}
	result := l.tempVar(el, hir.T(hir.Array, elem))
	body := append(rowPrelude, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp("array.push", result, hir.T(hir.I32), row)})
	keys := l.rtOp("map.keys", m, hir.T(hir.Array, key))
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(el), Name: k.Name, Type: key, X: keys, Body: hir.B(body...)})
	return result
}

// codePointValues splits a string as its iterator does: one element per code
// point, a well-formed surrogate pair staying together, a lone surrogate alone.
func (l *lowerer) codePointValues(at *ast.Node, s *hir.Expr) *hir.Expr {
	str, i, b := hir.T(hir.String), hir.T(hir.I32), hir.T(hir.Bool)
	bin := func(op string, x, y *hir.Expr, typ hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(at), Op: op, Type: typ, X: x, Y: y}
	}
	assign := func(x, y *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: x, Y: y} }
	s = l.tempInit(at, str, s)
	result := l.tempVar(at, hir.T(hir.Array, str))
	n := l.tempInit(at, i, l.rtOp("string.length", s, i))
	pos := l.tempInit(at, i, hir.L(i, 0))
	start := l.tempInit(at, i, hir.L(i, 0))
	code := l.tempInit(at, i, hir.L(i, 0))
	next := l.tempInit(at, i, hir.L(i, 0))
	high := bin("&&", bin(">=", code, hir.L(i, 0xD800), b), bin("<=", code, hir.L(i, 0xDBFF), b), b)
	low := bin("&&", bin(">=", next, hir.L(i, 0xDC00), b), bin("<=", next, hir.L(i, 0xDFFF), b), b)
	step := func() *hir.Stmt { return assign(pos, bin("+", pos, hir.L(i, 1), i)) }
	body := hir.B(
		assign(start, pos),
		assign(code, l.rtOp("string.charCodeAt", s, i, pos)),
		step(),
		&hir.Stmt{Kind: hir.If, Node: l.node(at), X: bin("&&", high, bin("<", pos, n, b), b), Body: hir.B(
			assign(next, l.rtOp("string.charCodeAt", s, i, pos)),
			&hir.Stmt{Kind: hir.If, Node: l.node(at), X: low, Body: hir.B(step())},
		)},
		&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(at), X: l.rtOp("array.push", result, i, l.rtOp("string.substring", s, str, start, pos))},
	)
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(at), X: bin("<", pos, n, b), Body: body})
	return result
}

func isMatchAllCall(n *ast.Node) bool {
	if n == nil || n.Kind != ast.KindCallExpression || len(n.Arguments()) != 1 {
		return false
	}
	callee := n.Expression()
	return callee != nil && callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "matchAll"
}

// matchAllValues consumes the same lazy iterator as direct calls and for-of.
func (l *lowerer) matchAllValues(n *ast.Node) *hir.Expr {
	recv := l.expr(n.Expression().Expression())
	if recv == nil {
		return nil
	}
	x := l.matchAllIterator(n, recv)
	if x == nil {
		return nil
	}
	return l.drainGenerator(x)
}

// convertedElements copies a tagged-element snapshot into the context's
// element type, each element converted as a TypeScript-typed value.
func (l *lowerer) convertedElements(at *ast.Node, src *hir.Expr, elem hir.Type) *hir.Expr {
	l.serial++
	v := hir.V("spreadValue"+itoa(l.serial), src.Type.Args[0])
	converted := l.coerce(v, elem)
	if converted == nil || !converted.Type.Equal(elem) {
		return src
	}
	result := l.tempVar(at, hir.T(hir.Array, elem))
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(at), Name: v.Name, Type: v.Type, X: src,
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(at), X: l.rtOp("array.push", result, hir.T(hir.I32), converted)})})
	return result
}
