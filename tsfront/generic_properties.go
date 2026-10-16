package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// keyof T & string is a string key. T[keyof T] is a tagged value when T
// ranges over objects; neither representation assumes a particular shape.
func genericObjectParameter(l *lowerer, t *checker.Type) bool {
	if t == nil || t.Flags()&checker.TypeFlagsTypeParameter == 0 {
		return false
	}
	c := l.ck.GetConstraintOfTypeParameter(t)
	return c != nil && c.Flags()&checker.TypeFlagsNonPrimitive != 0
}
func (l *lowerer) genericStringKey(t, object *checker.Type) bool {
	if t == nil || t.Flags()&checker.TypeFlagsIntersection == 0 {
		return false
	}
	key, str := false, false
	for _, p := range t.AsUnionOrIntersectionType().Types() {
		if p.Flags()&checker.TypeFlagsString != 0 {
			str = true
			continue
		}
		if p.Flags()&checker.TypeFlagsIndex != 0 && p.AsIndexType().Target() == object {
			key = true
			continue
		}
		return false
	}
	return key && str
}
func (l *lowerer) genericPropertyType(t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil {
		return hir.Type{}, false
	}
	if t.Flags()&checker.TypeFlagsIntersection != 0 {
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			if p.Flags()&checker.TypeFlagsIndex != 0 && genericObjectParameter(l, p.AsIndexType().Target()) && l.genericStringKey(t, p.AsIndexType().Target()) {
				return hir.T(hir.String), true
			}
		}
	}
	if t.Flags()&checker.TypeFlagsIndexedAccess != 0 {
		x := t.AsIndexedAccessType()
		obj, key := x.ObjectType(), x.IndexType()
		if genericObjectParameter(l, obj) && (key.Flags()&checker.TypeFlagsIndex != 0 && key.AsIndexType().Target() == obj || l.genericStringKey(key, obj)) {
			return hir.T(hir.Dynamic), true
		}
	}
	if t.Flags()&checker.TypeFlagsConditional == 0 || !genericObjectParameter(l, t.AsConditionalType().CheckType()) || t.AsConditionalType().ExtendsType().Flags()&checker.TypeFlagsNonPrimitive == 0 || t.Alias() == nil || t.Alias().Symbol() == nil {
		return hir.Type{}, false
	}
	for _, decl := range t.Alias().Symbol().Declarations {
		if decl.Kind != ast.KindTypeAliasDeclaration || decl.Type() == nil || decl.Type().Kind != ast.KindConditionalType {
			continue
		}
		c := decl.Type().AsConditionalTypeNode()
		if c.CheckType.Kind != ast.KindTypeReference || c.ExtendsType.Kind != ast.KindObjectKeyword || c.TrueType.Kind != ast.KindIndexedAccessType || c.FalseType.Kind != ast.KindNeverKeyword {
			continue
		}
		name := c.CheckType.AsTypeReferenceNode().TypeName.Text()
		same := func(n *ast.Node) bool {
			return n != nil && n.Kind == ast.KindTypeReference && n.AsTypeReferenceNode().TypeName.Text() == name && len(n.TypeArguments()) == 0
		}
		x := c.TrueType.AsIndexedAccessTypeNode()
		if same(x.ObjectType) && x.IndexType.Kind == ast.KindTypeOperator && x.IndexType.AsTypeOperatorNode().Operator == ast.KindKeyOfKeyword && same(x.IndexType.Type()) {
			return hir.T(hir.Dynamic), true
		}
	}
	return hir.Type{}, false
}

func (l *lowerer) genericPropertyRead(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || !genericObjectParameter(l, l.ck.GetTypeAtLocation(n.Expression())) {
		return nil, false
	}
	typ, ok := l.genericPropertyType(l.ck.GetTypeAtLocation(n))
	if !ok || typ.Kind != hir.Dynamic {
		return nil, false
	}
	obj, key := l.expr(n.Expression()), l.expr(n.AsElementAccessExpression().ArgumentExpression)
	if obj == nil || key == nil {
		return nil, true
	}
	if key.Type.Kind != hir.String {
		l.diagf(n, "unsupported-generic", "generic property access requires a checked string key")
		return nil, true
	}
	return l.rtOp("dynamic.get", l.coerce(obj, hir.T(hir.Dynamic)), hir.T(hir.Dynamic), key), true
}
