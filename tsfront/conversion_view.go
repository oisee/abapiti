package tsfront

import "github.com/oisee/abapiti/hir"

// Erased assignments and assertions use the same live storage boundary as
// erased interface arguments. Backends certify the complete class ancestry;
// a view never runs a constructor or invents prototype methods.
func (l *lowerer) dynamicReference(x *hir.Expr, target hir.Type) *hir.Expr {
	op := "dynamic.asRef"
	if l.unprunedFeatures && target.Kind == hir.ClassRef && target.Name != hir.RootObject {
		op = "dynamic.classView"
	}
	if l.unprunedFeatures && target.Kind == hir.Array && (target.Args[0].Kind == hir.String || target.Args[0].Kind == hir.ClassRef) {
		op = "dynamic.arrayView"
	}
	return l.rtOp(op, x, target)
}
