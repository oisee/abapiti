package tsfront

import (
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
