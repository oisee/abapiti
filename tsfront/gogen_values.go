package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Recursive primitive/record values have no finite field layout. The tagged
// value ABI preserves primitive tags, undefined, and the record's own keys.
// Recognize the resolved graph, including flattened unions and narrowed index
// records; do not depend on alias names or annotation spelling.
func (l *lowerer) gogenValueType(t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil {
		return hir.Type{}, false
	}
	if l.arrayRecordUnion(t) {
		return hir.T(hir.Dynamic), true
	}
	if l.gogenRecursiveArrayValue(t) {
		return hir.T(hir.Dynamic), true
	}
	parts := []*checker.Type{t}
	if t.IsUnion() {
		parts = t.AsUnionOrIntersectionType().Types()
	}
	for _, p := range parts {
		infos := l.ck.GetIndexInfosOfType(p)
		if len(infos) != 1 || len(l.ck.GetPropertiesOfType(p)) != 0 || infos[0].KeyType().Flags()&checker.TypeFlagsString == 0 {
			continue
		}
		seen := map[*checker.Type]bool{}
		recursive := false
		var valid func(*checker.Type) bool
		valid = func(v *checker.Type) bool {
			if seen[v] {
				recursive = true
				return true
			}
			if v.Flags()&(checker.TypeFlagsStringLike|checker.TypeFlagsNumberLike|checker.TypeFlagsUndefined) != 0 {
				return true
			}
			seen[v] = true
			defer delete(seen, v)
			if v.IsUnion() {
				for _, part := range v.AsUnionOrIntersectionType().Types() {
					if !valid(part) {
						return false
					}
				}
				return true
			}
			ix := l.ck.GetIndexInfosOfType(v)
			return len(ix) == 1 && len(l.ck.GetPropertiesOfType(v)) == 0 && ix[0].KeyType().Flags()&checker.TypeFlagsString != 0 && valid(ix[0].ValueType())
		}
		if valid(p) && recursive {
			return hir.T(hir.Dynamic), true
		}
	}
	return hir.Type{}, false
}

func (l *lowerer) gogenIndexedShape(t *checker.Type) bool {
	if !l.unprunedFeatures || t == nil {
		return false
	}
	ix := l.ck.GetIndexInfosOfType(t)
	return len(ix) == 1 && ix[0].KeyType().Flags()&checker.TypeFlagsString != 0 && ix[0].ValueType().Flags()&checker.TypeFlagsUnknown != 0 && len(l.ck.GetPropertiesOfType(t)) > 0
}

func (l *lowerer) gogenIndexedLiteral(n *ast.Node, target *hir.Class) *hir.Expr {
	record := l.dynamicRecordLiteral(n)
	if record == nil {
		return nil
	}
	return l.dynamicReference(record, hir.Ref(target.Name))
}

// A dynamic key can alias a named slot and invalidate its typed view. Until
// that alias policy is proven, all writes through a mixed index are refused.
func (l *lowerer) gogenIndexWrite(n *ast.Node) bool {
	if n.Kind != ast.KindElementAccessExpression && n.Kind != ast.KindPropertyAccessExpression {
		return false
	}
	if !l.gogenIndexedShape(l.ck.GetTypeAtLocation(n.Expression())) {
		return false
	}
	l.diagf(n, "unsupported-assignment", "writes through a mixed index signature require an exact named-slot alias policy")
	return true
}

func (l *lowerer) gogenValueProperty(n *ast.Node) (*hir.Expr, bool) {
	if _, ok := l.gogenValueType(l.ck.GetTypeAtLocation(n.Expression())); !ok && !l.arrayRecordProperty(n) {
		return nil, false
	}
	recv := l.expr(n.Expression())
	if recv == nil {
		return nil, true
	}
	return l.rtOp("dynamic.get", l.coerce(recv, hir.T(hir.Dynamic)), hir.T(hir.Dynamic), hir.L(hir.T(hir.String), n.Name().Text())), true
}

// Mapping a member descends from its instantiated collection type, even when
// the source node denotes an optional property rather than a Map annotation.
func (l *lowerer) gogenActiveMapValue(t *checker.Type) bool {
	for source := range l.mappingTypes {
		sym := source.Symbol()
		if sym == nil || !l.librarySymbol(sym) || (sym.Name != "Map" && sym.Name != "ReadonlyMap") {
			continue
		}
		args := l.ck.GetTypeArguments(source)
		if len(args) == 2 && args[1] == t {
			return true
		}
	}
	return false
}

func (l *lowerer) gogenResolvedMembers(node *ast.Node) bool {
	t := l.ck.GetDeclaredTypeOfSymbol(node.Symbol())
	if l.gogenIndexedShape(t) {
		return true
	}
	for _, p := range l.ck.GetPropertiesOfType(t) {
		pt := l.ck.GetTypeOfSymbol(p)
		if _, ok := l.gogenValueType(pt); ok {
			return true
		}
		parts := []*checker.Type{pt}
		if pt.IsUnion() {
			parts = pt.AsUnionOrIntersectionType().Types()
		}
		for _, part := range parts {
			sym := part.Symbol()
			if sym != nil && l.librarySymbol(sym) && (sym.Name == "Map" || sym.Name == "ReadonlyMap") {
				return true
			}
		}
	}
	return false
}

func (l *lowerer) gogenIndexRead(n *ast.Node) (*hir.Expr, bool) {
	if n.Kind != ast.KindElementAccessExpression && n.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	t := l.ck.GetTypeAtLocation(n.Expression())
	if !l.gogenIndexedShape(t) {
		return nil, false
	}
	var key *ast.Node
	if n.Kind == ast.KindPropertyAccessExpression {
		key = n.Name()
	} else {
		key = n.AsElementAccessExpression().ArgumentExpression
	}
	if key == nil {
		return nil, false
	}
	if (n.Kind == ast.KindPropertyAccessExpression || key.Kind == ast.KindStringLiteral) && l.ck.GetPropertyOfType(t, key.Text()) != nil {
		return nil, false
	}
	recv := l.expr(n.Expression())
	if recv == nil {
		return nil, true
	}
	recv = l.tempInit(n, recv.Type, recv)
	var k *hir.Expr
	if n.Kind == ast.KindPropertyAccessExpression {
		k = hir.L(hir.T(hir.String), key.Text())
	} else {
		k = l.expr(key)
	}
	if k == nil {
		return nil, true
	}
	if k.Type.Kind != hir.String {
		k = l.primitiveString(n, k)
		if k == nil {
			return nil, true
		}
	}
	return l.rtOp("dynamic.get", l.coerce(recv, hir.T(hir.Dynamic)), hir.T(hir.Dynamic), k), true
}

func (l *lowerer) gogenIndexedType(t *checker.Type) (hir.Type, bool) {
	if l.gogenIndexedShape(t) && t.Symbol() != nil {
		if c := l.classOf(t.Symbol()); c != nil {
			return hir.Ref(c.Name), true
		}
	}
	return hir.Type{}, false
}
