package tsfront

import "github.com/oisee/abapiti/hir"

// Array property reads do not apply ToIntegerOrInfinity: a fractional or
// nonfinite number names no element. Reuse the bounded index conversion only
// when it round-trips; -1 represents the absent property otherwise. This is
// separate from indexValue, which also serves slice/splice and writes.
func (l *lowerer) arrayReadIndex(x *hir.Expr) *hir.Expr {
	if !l.unprunedFeatures || x == nil || x.Type.Kind != hir.Number {
		return l.indexValue(x)
	}
	if r := numericLiteral(x); r.state == 1 && r.lo >= -2147483648 && r.hi <= 2147483647 {
		return l.indexValue(x)
	}
	// Keep conversion temporaries inside the index operand: the array
	// receiver must run first, and its original checked view stays visible.
	saved := l.pend
	l.pend = nil
	value := l.tempInit(nil, x.Type, x)
	index := l.tempInit(nil, hir.T(hir.I32), l.indexValue(value))
	back := &hir.Expr{Kind: hir.RuntimeOp, Op: "number.fromI32", Type: hir.T(hir.Number), X: index}
	result := &hir.Expr{Kind: hir.Conditional, Node: x.Node, Type: hir.T(hir.I32),
		X: &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: value, Y: back},
		Y: index, Z: hir.L(hir.T(hir.I32), -1)}
	pre := l.pend
	l.pend = saved
	return &hir.Expr{Kind: hir.Seq, Node: x.Node, Type: result.Type, Stmt: hir.B(pre...), Y: result}
}

func arrayIndexRead(x *hir.Expr) bool {
	for x != nil && x.Kind == hir.Seq {
		x = x.Y
	}
	return x != nil && x.X != nil && x.X.Type.Kind == hir.Array &&
		(x.Kind == hir.IndexGet || x.Kind == hir.RuntimeOp && x.Op == "array.get")
}

// Truthiness and strict equality observe undefined without dereferencing it.
// A scalar array annotation does not prove that an index exists at runtime.
// Keep this local to the observation: required reads elsewhere remain checked.
func (l *lowerer) observedArrayIndex(x *hir.Expr) *hir.Expr {
	if !l.unprunedFeatures || x == nil {
		return x
	}
	if x.Kind == hir.Seq {
		y := l.observedArrayIndex(x.Y)
		if y != x.Y {
			copy := *x
			copy.Y, copy.Type = y, y.Type
			return &copy
		}
		return x
	}
	if x.Kind != hir.IndexGet || x.X == nil || x.X.Type.Kind != hir.Array || x.Type.IsRef() || x.Type.Kind == hir.Optional {
		return x
	}
	return &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: "array.get", Type: hir.T(hir.Optional, x.Type), X: x.X, Args: []*hir.Expr{x.Y}}
}
