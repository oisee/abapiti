package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// mapForOf lowers `for (const [k, v] of map)` over a typed Map. JavaScript
// iterates the live entry list: a deleted entry not yet reached is skipped, a
// value replaced before it is reached is read current, and an entry appended
// during the loop (a new key, or a deleted key set again) is visited later.
// The loop walks a key snapshot, skips keys no longer present and reads each
// value at its visit, which is exact while nothing is appended; the map's
// insertion counter is checked before every visit and after the loop, so an
// append traps at the first point where the two iterations would diverge.
func (l *lowerer) mapForOf(n, decl *ast.Node, m *hir.Expr) (*hir.Stmt, bool) {
	pattern := decl.Name()
	if pattern == nil || pattern.Kind != ast.KindArrayBindingPattern || len(m.Type.Args) != 2 {
		return nil, false
	}
	keyT, valT := m.Type.Args[0], m.Type.Args[1]
	var names [2]*ast.Node
	for i, el := range pattern.AsBindingPattern().Elements.Nodes {
		if el.Kind == ast.KindOmittedExpression || el.Kind == ast.KindBindingElement && el.Name() == nil && el.AsBindingElement().DotDotDotToken == nil {
			continue
		}
		be := el.AsBindingElement()
		if i > 1 || el.Kind != ast.KindBindingElement || be.DotDotDotToken != nil || be.Initializer != nil || el.Name() == nil || el.Name().Kind != ast.KindIdentifier {
			l.diagf(el, "unsupported-binding", "Map for-of binds only [key, value] identifiers")
			return nil, true
		}
		names[i] = el.Name()
	}
	f := n.AsForInOrOfStatement()
	m = l.tempInit(f.Expression, m.Type, m)
	i32 := hir.T(hir.I32)
	keys := l.tempInit(f.Expression, hir.T(hir.Array, keyT), l.rtOp("map.keys", m, hir.T(hir.Array, keyT)))
	insertions := func() *hir.Expr {
		x := l.rtOp("map.insertions", m, i32)
		if x.Op == "number.fromI32" {
			x = x.X // compare the native counters
		}
		x.Node = l.node(n)
		return x
	}
	inserted := l.tempInit(f.Expression, i32, insertions())
	guard := func() *hir.Stmt {
		changed := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "!=", X: insertions(), Y: inserted}
		return &hir.Stmt{Kind: hir.If, Node: l.node(n), X: changed,
			Body: hir.B(&hir.Stmt{Kind: hir.Trap, Node: l.node(n), Name: l.trapLocation(n) + ": Map for-of appended an entry during iteration"})}
	}
	l.serial++
	key := hir.V("k"+itoa(l.serial), keyT)
	head := []*hir.Stmt{guard(),
		{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "!", X: l.rtOp("map.has", m, hir.T(hir.Bool), key)},
			Body: hir.B(&hir.Stmt{Kind: hir.Continue, Node: l.node(n)})}}
	l.push()
	l.declare(key.Name, keyT)
	for i, name := range names {
		if name == nil {
			continue
		}
		typ, value := keyT, key
		if i == 1 {
			// A stored undefined reads as undefined; a required value is
			// present because the key was just found.
			typ = valT
			if valT.Kind == hir.Optional {
				value = l.rtOp("map.get", m, valT, key)
			} else {
				value = &hir.Expr{Kind: hir.Narrow, Node: l.node(name), Type: valT, X: l.rtOp("map.get", m, hir.T(hir.Optional, valT), key)}
			}
		}
		l.declare(name.Text(), typ)
		d := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(name), Name: name.Text(), Type: typ, X: value}
		l.recordPatternBinding(name, d)
		head = append(head, d)
	}
	body := l.scopeBlock(f.Statement)
	l.pop()
	if body == nil {
		return nil, true
	}
	loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: keyT, X: keys, Body: hir.B(append(head, body)...)}
	return hir.B(loop, guard()), true
}

// The empty-array arm is only observed by iteration here. An internal empty
// Map has the same empty sequence and lets the present arm keep live Map
// iteration, including its located append refusal.
func (l *lowerer) emptyMapFallbackForOf(n, decl *ast.Node) (*hir.Stmt, bool) {
	if !l.unprunedFeatures || decl.Name().Kind != ast.KindArrayBindingPattern {
		return nil, false
	}
	expr := n.AsForInOrOfStatement().Expression
	for expr.Kind == ast.KindParenthesizedExpression {
		expr = expr.Expression()
	}
	if expr.Kind != ast.KindBinaryExpression {
		return nil, false
	}
	b := expr.AsBinaryExpression()
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
	if typ.Kind != hir.OrderedMap {
		return nil, false
	}
	m := l.expr(b.Left)
	if m == nil {
		return nil, true
	}
	if m.Type.Kind == hir.Dynamic || m.Type.Kind == hir.Optional && m.Type.Args[0].Kind == hir.Dynamic {
		m = l.coerce(m, hir.T(hir.Optional, typ))
	}
	if m.Type.Kind == hir.Optional {
		m = l.tempInit(b.Left, m.Type, m)
		typ = m.Type.Args[0]
		m = &hir.Expr{Kind: hir.Conditional, Node: l.node(expr), Type: typ, X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(expr), Type: hir.T(hir.Bool), X: m}, Y: &hir.Expr{Kind: hir.New, Node: l.node(expr), Type: typ}, Z: &hir.Expr{Kind: hir.Narrow, Node: l.node(expr), Type: typ, X: m}}
	}
	if m.Type.Kind != hir.OrderedMap {
		l.diagf(expr, "unsupported-statement", "Map fallback loop needs typed Map storage")
		return nil, true
	}
	return l.mapForOf(n, decl, m)
}
