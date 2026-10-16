package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// An array intersected with optional data properties keeps its native array
// identity. Record-or-array unions use the tagged ABI rather than inventing a
// common nominal layout for the array's prototype and the record's fields.
func (l *lowerer) arrayRecordElement(t *checker.Type) *checker.Type {
	if !l.unprunedFeatures || t == nil || t.Flags()&checker.TypeFlagsIntersection == 0 {
		return nil
	}
	var element *checker.Type
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		if l.ck.IsArrayType(part) {
			args := l.ck.GetTypeArguments(part)
			if len(args) != 1 || element != nil {
				return nil
			}
			element = args[0]
		} else {
			if part.Flags()&checker.TypeFlagsObject == 0 || l.ck.TypeHasCallOrConstructSignatures(part) || len(l.ck.GetIndexInfosOfType(part)) != 0 {
				return nil
			}
			for _, p := range l.ck.GetPropertiesOfType(part) {
				if p.Flags&ast.SymbolFlagsOptional == 0 {
					return nil
				}
			}
		}
	}
	return element
}
func (l *lowerer) arrayRecordUnion(t *checker.Type) bool {
	if !l.unprunedFeatures || t == nil || !t.IsUnion() {
		return false
	}
	array, record := false, false
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		if l.arrayRecordElement(part) != nil {
			array = true
			continue
		}
		if part.Flags()&checker.TypeFlagsObject == 0 || l.ck.IsArrayType(part) || l.ck.TypeHasCallOrConstructSignatures(part) || len(l.ck.GetIndexInfosOfType(part)) != 0 {
			return false
		}
		record = true
	}
	return array && record
}
func (l *lowerer) arrayRecordProperty(n *ast.Node) bool {
	if n == nil || n.Kind != ast.KindPropertyAccessExpression || l.arrayRecordElement(l.ck.GetTypeAtLocation(n.Expression())) == nil {
		return false
	}
	t := l.ck.GetTypeAtLocation(n.Expression())
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		if !l.ck.IsArrayType(part) && l.ck.GetPropertyOfType(part, n.Name().Text()) != nil {
			return true
		}
	}
	return false
}
func (l *lowerer) arrayRecordWrite(n *ast.Node) bool {
	if !l.arrayRecordProperty(n) {
		return false
	}
	l.diagf(n, "unsupported-assignment", "named metadata writes on native array intersections are not lowered")
	return true
}
func (l *lowerer) arrayRecordType(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if element := l.arrayRecordElement(t); element != nil {
		return hir.T(hir.Array, l.mapCheckerType(n, element)), true
	}
	return hir.Type{}, false
}
