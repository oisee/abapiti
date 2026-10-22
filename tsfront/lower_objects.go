package tsfront

import (
	"strconv"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// assignRecords preserves target identity and snapshots each source's own keys.
// Map entries are not JS own properties, despite sharing the record ABI.
func (l *lowerer) assignRecords(n *ast.Node) *hir.Expr {
	args := n.Arguments()
	if len(args) == 0 {
		l.diagf(n, "unsupported-call", "Object.assign needs a record target")
		return nil
	}
	if l.unprunedFeatures && l.mapCheckerType(args[0], l.ck.GetTypeAtLocation(args[0])).Kind == hir.ClassRef {
		return l.assignClassRecords(n)
	}
	if len(args) == 2 {
		if lit := ast.SkipParentheses(args[1]); lit.Kind == ast.KindObjectLiteralExpression {
			if x, handled := l.assignLiteralFields(n, args[0], lit); handled {
				return x
			}
		}
	}
	var values []*hir.Expr
	var targetType hir.Type
	for index, arg := range args {
		value := l.expr(arg)
		if value == nil {
			return nil
		}
		typ := value.Type
		if typ.Kind == hir.Optional {
			typ = typ.Args[0]
		}
		checked := l.ck.GetNonNullableType(l.ck.GetTypeAtLocation(arg))
		isMap := checked != nil && checked.Symbol() != nil && (checked.Symbol().Name == "Map" || checked.Symbol().Name == "ReadonlyMap")
		if isMap || typ.Kind != hir.OrderedMap || typ.Args[0].Kind != hir.String || index > 0 && !typ.Equal(targetType) {
			l.diagf(n, "unsupported-call", "Object.assign supports matching string-keyed records only")
			return nil
		}
		if index == 0 {
			targetType = typ
		}
		// Evaluate every argument before copying any properties.
		values = append(values, l.tempInit(arg, value.Type, value))
	}
	target := values[0]
	if target.Type.Kind == hir.Optional {
		// JS evaluates all arguments before ToObject rejects an absent target.
		target = l.tempInit(n, targetType, &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: targetType, X: target})
	}
	for _, source := range values[1:] {
		recv := source
		if recv.Type.Kind == hir.Optional {
			recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[0], X: recv}
		}
		l.serial++
		key := hir.V("k"+itoa(l.serial), hir.T(hir.String))
		valueType := target.Type.Args[1]
		value := l.rtOp("map.get", recv, hir.T(hir.Optional, valueType), key)
		value = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: valueType, X: value}
		copy := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: key.Type,
			X: l.rtOp("map.keys", recv, hir.T(hir.Array, key.Type)), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("map.set", target, target.Type, key, value)})}
		if source.Type.Kind == hir.Optional {
			copy = &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: source}}, Body: hir.B(copy)}
		}
		l.pendStmt(copy)
	}
	return target
}

// assignLiteralFields lowers Object.assign(target, {k: v, ...}) onto a typed
// record target. The literal's own keys are its plain data properties, in
// source order (no array-index keys, duplicates, spreads, accessors, methods
// or computed keys, so no symbols and no getters). JS evaluates the target,
// then every value, then [[Set]]s each key left to right and returns the
// target: exactly `t.k = v` per key, each key a declared data field.
func (l *lowerer) assignLiteralFields(n, targetNode, lit *ast.Node) (*hir.Expr, bool) {
	checked := l.ck.GetTypeAtLocation(targetNode)
	if l.mapCheckerType(targetNode, checked).Kind != hir.ClassRef {
		return nil, false
	}
	// The call's type is target & literal, a different record shape; its
	// value (the target) is exact only where the result is discarded or
	// that intersection maps to the target's own type.
	if parent := n.Parent; (parent == nil || parent.Kind != ast.KindExpressionStatement) && !l.mapCheckerType(n, l.ck.GetTypeAtLocation(n)).Equal(l.mapCheckerType(targetNode, checked)) {
		l.diagf(n, "unsupported-call", "Object.assign onto a typed record: only a discarded result is lowered")
		return nil, true
	}
	type prop struct {
		key   string
		value *ast.Node
		at    *ast.Node
	}
	var props []prop
	seen := map[string]bool{}
	for _, el := range lit.AsObjectLiteralExpression().Properties.Nodes {
		var key string
		var value *ast.Node
		switch el.Kind {
		case ast.KindPropertyAssignment:
			name := el.Name()
			if name.Kind != ast.KindIdentifier && name.Kind != ast.KindStringLiteral {
				l.diagf(el, "unsupported-call", "Object.assign onto a typed record needs plain literal keys")
				return nil, true
			}
			key, value = name.Text(), el.Initializer()
		case ast.KindShorthandPropertyAssignment:
			if el.AsShorthandPropertyAssignment().ObjectAssignmentInitializer != nil {
				l.diagf(el, "unsupported-call", "Object.assign onto a typed record needs plain literal keys")
				return nil, true
			}
			key, value = el.Name().Text(), el.Name()
		default:
			l.diagf(el, "unsupported-call", "Object.assign onto a typed record supports data properties only, not %s", el.Kind.String())
			return nil, true
		}
		if _, err := strconv.ParseUint(key, 10, 64); key == "__proto__" || seen[key] || err == nil {
			l.diagf(el, "unsupported-call", "Object.assign key %q changes own-key order or the prototype", key)
			return nil, true
		}
		seen[key] = true
		p := l.ck.GetPropertyOfType(checked, key)
		if p == nil || p.Flags&ast.SymbolFlagsProperty == 0 || p.Flags&ast.SymbolFlagsAccessor != 0 {
			l.diagf(el, "unsupported-call", "Object.assign key %s is not a declared data property of the target", key)
			return nil, true
		}
		props = append(props, prop{key, value, el})
	}
	target := l.expr(targetNode)
	if target == nil {
		return nil, true
	}
	target = l.tempInit(targetNode, target.Type, target)
	var writes []*hir.Stmt
	for _, p := range props {
		field := l.shapeWriteTarget(p.at, target, p.key)
		if field == nil || field.Kind != hir.FieldGet {
			l.diagf(p.at, "unsupported-call", "Object.assign key %s is not a field of the target record", p.key)
			return nil, true
		}
		old := l.hint
		l.hint = field.Type
		v := l.expr(p.value)
		l.hint = old
		if v == nil {
			return nil, true
		}
		v = l.tempInit(p.at, v.Type, v)
		writes = append(writes, &hir.Stmt{Kind: hir.Assign, Node: l.node(p.at), X: field, Y: l.coerce(v, field.Type)})
	}
	for _, w := range writes {
		l.pendStmt(w)
	}
	return target, true
}

// assignClassRecords uses the existing live own-property view. Arguments are
// evaluated before copying; each source's enumerable keys are snapshotted
// before its values are read, and writes retain target identity.
func (l *lowerer) assignClassRecords(n *ast.Node) *hir.Expr {
	args := n.Arguments()
	values := make([]*hir.Expr, 0, len(args))
	for _, arg := range args {
		value := l.expr(arg)
		if value == nil {
			return nil
		}
		values = append(values, l.tempInit(arg, value.Type, value))
	}
	target := values[0]
	dyn := hir.T(hir.Dynamic)
	boxed := l.tempInit(n, dyn, l.coerce(target, dyn))
	for i, source := range values[1:] {
		source = l.tempInit(args[i+1], dyn, l.coerce(source, dyn))
		l.serial++
		key := hir.V("assignKey"+itoa(l.serial), hir.T(hir.String))
		keys := l.rtOp("dynamic.keys", source, hir.T(hir.Array, key.Type))
		value := l.rtOp("dynamic.get", source, dyn, key)
		put := l.rtOp("dynamic.put", boxed, hir.T(hir.Void), key, value)
		body := hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: put})
		for c := l.classByName(target.Type.Name); c != nil; c = l.classByName(c.Super) {
			for _, field := range c.Fields {
				if field.Static {
					continue
				}
				write := &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Type: field.Type, X: target, Name: field.Name}, Y: l.coerce(value, field.Type)}
				body = hir.B(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: key, Y: hir.L(key.Type, field.Name)}, Body: hir.B(write), Else: body})
			}
		}
		copy := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: key.Type, X: keys, Body: body}
		l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: l.rtOp("dynamic.isNullish", source, hir.T(hir.Bool))}, Body: hir.B(copy)})
	}
	return target
}
