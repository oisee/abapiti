package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// stringIndexOfFrom lowers the numeric start-position overload through existing
// UTF-16 substring/search operations. Evaluate receiver and both arguments once
// in source order. number.index supplies ToIntegerOrInfinity for finite Numbers;
// clamp to [0,length] before searching. Failed searches must keep -1, while an
// empty needle returns the clamped position even at the end of the string.
func (l *lowerer) stringIndexOfFrom(n *ast.Node, receiver *hir.Expr, args []*ast.Node) *hir.Expr {
	str, i, b, num := hir.T(hir.String), hir.T(hir.I32), hir.T(hir.Bool), hir.T(hir.Number)
	recv := l.tempInit(n, str, receiver)
	needle := l.expr(args[0])
	if needle == nil {
		return nil
	}
	if needle.Type.Kind != hir.String {
		l.diagf(n, "unsupported-call", "string.indexOf start-position overload requires a string search value")
		return nil
	}
	needle = l.tempInit(n, str, needle)
	position := l.expr(args[1])
	if position == nil {
		return nil
	}
	if position.Type.Kind == hir.Optional && position.Type.Args[0].Kind == hir.Number {
		position = l.tempInit(n, position.Type, position)
		position = &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: num, X: &hir.Expr{Kind: hir.IsUndefined, Type: b, X: position}, Y: hir.L(num, 0), Z: &hir.Expr{Kind: hir.Narrow, Type: num, X: position}}
	} else if l.isUndefinedType(args[1]) && position.Kind == hir.Lit {
		position = hir.L(num, 0)
	}
	if position.Type.Kind != hir.Number && position.Type.Kind != hir.I32 {
		l.diagf(n, "unsupported-call", "string.indexOf start position requires a Number or undefined")
		return nil
	}
	position = l.tempInit(n, i, l.indexValue(position))
	length := l.tempInit(n, i, &hir.Expr{Kind: hir.RuntimeOp, Node: l.node(n), Op: "string.length", Type: i, X: recv})
	binary := func(op string, x, y *hir.Expr, t hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: t, X: x, Y: y}
	}
	start := l.tempInit(n, i, &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: i, X: binary("<", position, hir.L(i, 0), b), Y: hir.L(i, 0), Z: &hir.Expr{Kind: hir.Conditional, Type: i, X: binary(">", position, length, b), Y: length, Z: position}})
	suffix := l.rtOp("string.substring", recv, str, start, length)
	found := l.tempInit(n, num, l.rtOp("string.indexOf", suffix, i, needle))
	offset := l.rtOp("number.fromI32", start, num)
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: num, X: binary("<", found, hir.L(num, 0), b), Y: found, Z: binary("+", found, offset, num)}
}
