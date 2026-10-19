package tsfront

import "github.com/oisee/abapiti/hir"

// Erased assignments and assertions use the same live storage boundary as
// erased interface arguments. Backends certify the complete class ancestry;
// a view never runs a constructor or invents prototype methods.
func (l *lowerer) dynamicReference(x *hir.Expr, target hir.Type) *hir.Expr {
	op := "dynamic.asRef"
	if l.unprunedFeatures && target.Kind == hir.OrderedMap && target.JSMap && target.Args[0].Kind == hir.String {
		op = "dynamic.mapView"
	}
	if l.unprunedFeatures && target.Equal(hir.Ref(hir.RootObject)) {
		op = "dynamic.asObject"
	}
	if l.unprunedFeatures && target.Kind == hir.ClassRef && target.Name != hir.RootObject {
		op = "dynamic.classView"
	}
	if l.unprunedFeatures && target.Kind == hir.Array && (target.Args[0].Kind == hir.String || target.Args[0].Kind == hir.ClassRef) {
		op = "dynamic.arrayView"
	}
	return l.rtOp(op, x, target)
}

// The object root carries original storage, not the nominal layout of a public
// property shape. A checked data view preserves that storage and its identity.
// Tagged-family members retain their separate membership projection boundary.
func (l *lowerer) rootObjectDataView(x *hir.Expr, target hir.Type) *hir.Expr {
	if !l.unprunedFeatures || !x.Type.Equal(hir.Ref(hir.RootObject)) || target.Kind != hir.ClassRef || !l.shapeLike[target.Name] || !hir.DataClassView(l.out, target.Name) {
		return nil
	}
	for _, family := range l.out.Unions {
		if family != nil {
			for _, member := range family.Alternatives {
				if member.Payload.Equal(target) {
					return nil
				}
			}
		}
	}
	return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), target)
}
