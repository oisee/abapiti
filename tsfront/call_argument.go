package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// callArgument is shared by named and stored-function calls. Structural data
// parameters can contain callable slots with different (compatible) signatures;
// their nominal HIR layouts need the same checked live view as an assertion.
func (l *lowerer) callArgument(call, at *ast.Node, index int, value *hir.Expr, target hir.Type) *hir.Expr {
	value = l.coerce(value, target)
	if !l.unprunedFeatures || value == nil || value.Type.Equal(target) || target.Kind != hir.ClassRef || !l.shapeLike[target.Name] || value.Type.Kind != hir.ClassRef || !l.shapeLike[value.Type.Name] || !hir.DataClassView(l.out, target.Name) {
		return value
	}
	sig := l.ck.GetResolvedSignature(call)
	if sig == nil || index >= len(sig.Parameters()) {
		return value
	}
	sourceType, targetType := l.ck.GetTypeAtLocation(at), l.ck.GetTypeOfSymbol(sig.Parameters()[index])
	if sourceType == nil || targetType == nil || sourceType.Flags()&checker.TypeFlagsAnyOrUnknown != 0 || !l.ck.IsTypeAssignableTo(sourceType, targetType) {
		return value
	}
	// The source signature must describe this physical parameter too (an erased
	// or specialized ABI may instead require its existing conversion).
	if !l.mapCheckerType(at, targetType).Equal(target) {
		return value
	}
	return l.dynamicReference(l.coerce(value, hir.T(hir.Dynamic)), target)
}
