package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

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
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.of" && l.hasReadonlyCovariance(target, x.X.Type, map[string]bool{}) {
			op = "container.readonlyClassView"
		}
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

// A checker-accepted assertion from a class or interface value (or T |
// undefined of one) to a comparable data shape erases in JS: reads reach the
// same object. Lower it as the live structural view over that storage, never
// a nominal downcast to the shape's projection layout, which no class value
// has. Nominal relatives keep their cast boundary; family payloads keep their
// membership projection; the view's reads keep the retained-domain checks.
func (l *lowerer) classShapeView(n *ast.Node, x *hir.Expr, target hir.Type) *hir.Expr {
	if !l.unprunedFeatures || target.Kind != hir.ClassRef || target.Name == hir.RootObject || !l.shapeLike[target.Name] || !hir.DataClassView(l.out, target.Name) {
		return nil
	}
	source := x.Type
	if source.Kind == hir.Optional {
		source = source.Args[0]
	}
	switch source.Kind {
	case hir.ClassRef:
		if source.Name == hir.RootObject || source.Equal(target) || l.acceptsType(target, source) || l.acceptsType(source, target) || l.commonAncestor(source.Name, target.Name) != "" {
			return nil
		}
	case hir.InterfaceRef:
		for _, i := range l.out.Interfaces {
			if i.Name == source.Name && len(i.StructuralMembers) != 0 {
				return nil
			}
		}
	default:
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
	from, to := l.ck.GetTypeAtLocation(n.Expression()), l.ck.GetTypeAtLocation(n)
	if from == nil || to == nil || from.Flags()&checker.TypeFlagsAnyOrUnknown != 0 || !l.ck.AreTypesComparable(l.ck.GetWidenedType(from), to) {
		return nil
	}
	return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), target)
}
