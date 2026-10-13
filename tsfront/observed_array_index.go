package tsfront

import "github.com/oisee/abapiti/hir"

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
