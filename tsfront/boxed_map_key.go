package tsfront

import "github.com/oisee/abapiti/hir"

// An erased Map key observes undefined as a value. The checker's unchecked
// array type (including a ?? fallback) cannot justify unwrapping absence.
// Keep this at the boxed-key consumer; required reads elsewhere stay checked.
func observedBoxedMapKey(x *hir.Expr) *hir.Expr {
	if x == nil {
		return x
	}
	if x.Kind == hir.Seq {
		y := observedBoxedMapKey(x.Y)
		if y != x.Y {
			copy := *x
			copy.Y, copy.Type = y, y.Type
			return &copy
		}
	}
	if x.Kind == hir.Narrow && x.X != nil && x.X.Type.Kind == hir.Optional && x.X.Type.Args[0].Equal(x.Type) {
		return x.X
	}
	return x
}
