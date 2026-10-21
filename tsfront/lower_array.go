package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Empty literals allocate the same array as nonempty literals. Their element
// type comes from the surrounding expression, never from a nonexistent element.
func (l *lowerer) emptyArrayLiteral(n *ast.Node) *hir.Expr {
	typ := l.hint
	if typ.Kind == hir.Optional && len(typ.Args) == 1 {
		typ = typ.Args[0]
	}
	if typ.Kind != hir.Array && typ.Kind != hir.OrderedSet {
		if t := l.ck.GetContextualType(n, 0); t != nil {
			if arr := l.singleArrayConstituent(t); arr != nil {
				t = arr
			}
			typ = l.mapCheckerType(n, t)
		}
		if typ.Kind == hir.Optional && len(typ.Args) == 1 {
			typ = typ.Args[0]
		}
	}
	if typ.Kind == hir.Dynamic {
		// An any/unknown context accepts arrays of tagged elements. Construct
		// a real fresh array; the enclosing expression handles its boxing.
		typ = hir.T(hir.Array, hir.T(hir.Dynamic))
	}
	if typ.Kind == hir.Array || typ.Kind == hir.OrderedSet {
		return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}
	}
	l.diagf(n, "unsupported-array", "empty array literal needs an inferable contextual element type")
	return nil
}

// A fresh [] fallback can use the checker's proven non-null array ABI even
// when the nullable source needs tagged storage. Only the selected present
// branch is unboxed; no existing array storage is copied or reinterpreted.
func (l *lowerer) emptyArrayFallbackBase(left, right *ast.Node, base hir.Type) hir.Type {
	if base.Kind != hir.Dynamic || right.Kind != ast.KindArrayLiteralExpression || len(right.AsArrayLiteralExpression().Elements.Nodes) != 0 {
		return base
	}
	typ := l.conditionalCheckerType(left, l.ck.GetNonNullableType(l.ck.GetTypeAtLocation(left)))
	if typ.Kind == hir.Array {
		return typ
	}
	return base
}

// A checker-erased array can flow into a contextual string[] slot (and back).
// These are the element domains supported by the checked live array runtime;
// other primitive or tuple element layouts still require their own proof.
func (l *lowerer) erasedStringArrayView(src, dst hir.Type) bool {
	if !l.unprunedFeatures || src.Kind != hir.Array || dst.Kind != hir.Array || len(src.Args) != 1 || len(dst.Args) != 1 {
		return false
	}
	a, b := src.Args[0].Kind, dst.Args[0].Kind
	return a == hir.Dynamic && b == hir.String || a == hir.String && b == hir.Dynamic
}
