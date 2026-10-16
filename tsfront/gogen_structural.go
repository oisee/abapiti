package tsfront

import (
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Recursive inspection aliases contain both primitive values and arrays of
// themselves. Their finite storage is the tagged JS graph, not a nominal row.
// Test the resolved array edge, so conditional/mapped aliases need no spelling
// special case and ordinary arrays and discriminated families keep their ABI.
func (l *lowerer) gogenRecursiveArrayValue(t *checker.Type) bool {
	if !l.unprunedFeatures || t == nil || !t.IsUnion() {
		return false
	}
	primitive, recursive := false, false
	for _, p := range t.AsUnionOrIntersectionType().Types() {
		primitive = primitive || p.Flags()&(checker.TypeFlagsStringLike|checker.TypeFlagsNumberLike|checker.TypeFlagsBooleanLike) != 0
		if l.ck.IsArrayType(p) {
			if info := l.ck.GetIndexInfoOfType(p, l.ck.GetNumberType()); info != nil {
				recursive = recursive || info.ValueType() == t
			}
		}
	}
	return primitive && recursive
}

func (l *lowerer) gogenResolvedAnnotation(n *ast.Node) (hir.Type, bool) {
	if !l.unprunedFeatures || n.Kind != ast.KindTypeReference {
		return hir.Type{}, false
	}
	t := l.ck.GetTypeFromTypeNode(n)
	if !resolvedGenericAlias(t) || l.librarySymbol(t.Alias().Symbol()) {
		return hir.Type{}, false
	}
	return l.mapCheckerType(n, t), true
}

func resolvedGenericAlias(t *checker.Type) bool {
	if t == nil || t.Alias() == nil || t.Alias().Symbol() == nil {
		return false
	}
	for _, d := range t.Alias().Symbol().Declarations {
		if d.Kind == ast.KindTypeAliasDeclaration && len(d.TypeParameters()) > 0 {
			return true
		}
	}
	return false
}

func (l *lowerer) gogenGenericShape(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || l.ck.IsArrayType(t) || len(l.ck.GetIndexInfosOfType(t)) != 0 {
		return hir.Type{}, false
	}
	if t.Flags()&checker.TypeFlagsIntersection != 0 {
		for _, part := range t.AsUnionOrIntersectionType().Types() {
			if part.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsIntersection) == 0 || len(l.ck.GetSignaturesOfType(part, checker.SignatureKindCall)) != 0 {
				return hir.Type{}, false
			}
		}
	} else if !resolvedGenericAlias(t) || t.Flags()&checker.TypeFlagsObject == 0 {
		return hir.Type{}, false
	}
	props := l.ck.GetPropertiesOfType(t)
	if t.Flags()&checker.TypeFlagsIntersection != 0 {
		props = append([]*ast.Symbol(nil), props...)
		sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
		slots := map[string]hir.Type{}
		for _, p := range props {
			method := false
			for _, d := range p.Declarations {
				method = method || d.Kind == ast.KindMethodSignature || d.Kind == ast.KindMethodDeclaration
			}
			if !method {
				continue
			}
			pt := l.ck.GetTypeOfSymbol(p)
			if fallback, ok := l.dataMemberFallback(n, pt); ok {
				slots[p.Name] = fallback
			} else if len(l.ck.GetSignaturesOfType(pt, checker.SignatureKindCall)) > 0 && len(l.ck.GetSignaturesOfType(pt, checker.SignatureKindConstruct)) == 0 {
				slots[p.Name] = l.functionType(n, pt)
			}
		}
		if c := l.synthFromPropertyTypes(n, props, slots); c != nil {
			return hir.Ref(c.Name), true
		}
		return hir.Type{}, false
	}
	if c := l.synthFromProperties(n, l.ck.GetPropertiesOfType(t)); c != nil {
		return hir.Ref(c.Name), true
	}
	return hir.Type{}, false
}

// Canonical property order determines only the layout hash. Runtime own keys
// and presence come from the original literal's own-key metadata.
func (l *lowerer) gogenStructuralShape(n *ast.Node, props []*ast.Symbol) (*hir.Class, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	props = append([]*ast.Symbol(nil), props...)
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	before := len(l.diags)
	c := l.synthFromPropertyTypes(n, props, nil)
	if c == nil {
		slots := map[string]hir.Type{}
		for _, p := range props {
			if l.isNeverArray(l.ck.GetTypeOfSymbol(p)) {
				slots[p.Name] = hir.T(hir.Array, hir.T(hir.Dynamic))
			}
		}
		if len(slots) > 0 {
			l.diags = l.diags[:before]
			c = l.synthFromPropertyTypes(n, props, slots)
		}
	}
	if c != nil && strings.HasPrefix(c.Name, "shape.") {
		if l.shapeLike == nil {
			l.shapeLike = map[string]bool{}
		}
		l.shapeLike[c.Name] = true
	}
	return c, true
}

func (l *lowerer) gogenStructuralArguments(target *hir.Class) bool {
	return l.unprunedFeatures && strings.HasPrefix(target.Name, "shape.")
}

func (l *lowerer) gogenStructuralPresence(n *ast.Node, obj, key *hir.Expr) *hir.Expr {
	if !l.unprunedFeatures || key.Type.Kind != hir.String {
		return nil
	}
	shape := obj.Type.Kind == hir.ClassRef && strings.HasPrefix(obj.Type.Name, "shape.")
	shape = shape || l.gogenObjectUnion(obj.Type)
	if !shape {
		return nil
	}
	// Heterogeneous union interfaces retain the original object and its own
	// keys. A presence test must inspect that storage, not the common layout.
	return l.rtOp("object.has", &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: hir.Ref(hir.RootObject), X: obj}, hir.T(hir.Bool), key)
}

func (l *lowerer) gogenAbsentShapeField(f hir.Field, src *hir.Class) bool {
	if !l.unprunedFeatures || f.Type.Kind != hir.Optional {
		return false
	}
	for _, g := range src.Fields {
		if g.Name == f.Name {
			return false
		}
	}
	return true
}

// Null is a tagged value; an explicitly undefined property still exists.
// Neither may be replaced by a fabricated root-object reference.
func (l *lowerer) gogenNullishProperty(t *checker.Type) (hir.Type, bool) {
	if l.unprunedFeatures && t != nil && t.Flags()&checker.TypeFlagsNull != 0 {
		return hir.T(hir.Dynamic), true
	}
	if l.unprunedFeatures && t != nil && t.Flags()&checker.TypeFlagsUndefined != 0 {
		return hir.T(hir.Optional, hir.T(hir.Dynamic)), true
	}
	return hir.Type{}, false
}

func (l *lowerer) gogenOptionalDynamicField(dst, src hir.Type) bool {
	return l.unprunedFeatures && dst.Kind == hir.Optional && src.Kind == hir.Optional && src.Args[0].Kind == hir.Dynamic
}

// Record enumeration uses JS integer-key ordering. Snapshot only the keys
// into the existing own-key runtime; record values and identity stay untouched.
func (l *lowerer) gogenRecordKeys(n *ast.Node, x *hir.Expr) *hir.Expr {
	if !l.unprunedFeatures || x.Type.Kind != hir.OrderedMap || x.Type.Args[0].Kind != hir.String {
		return nil
	}
	t := l.ck.GetTypeAtLocation(n.Arguments()[0])
	if t == nil || len(l.ck.GetIndexInfosOfType(t)) != 1 {
		return nil
	}
	str, dyn := hir.T(hir.String), hir.T(hir.Dynamic)
	record := l.tempInit(n, dyn, l.rtOp("json.parseSubset", hir.L(str, "{}"), dyn))
	l.serial++
	key := hir.V("key"+itoa(l.serial), str)
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: str, X: l.rtOp("map.keys", x, hir.T(hir.Array, str)), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("dynamic.put", record, hir.T(hir.Void), key, l.coerce(hir.L(hir.T(hir.Bool), false), dyn))})})
	return l.rtOp("dynamic.keys", record, hir.T(hir.Array, str))
}

// A mapped recursive object graph has no nominal declaration to re-enter.
// Keep its resolved outer fields typed and break only the recursive storage
// edge with the tagged value ABI, retaining object identity and own properties.
func (l *lowerer) gogenRecursiveStructuralType(t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures {
		return hir.Type{}, false
	}
	if t.Symbol() != nil {
		if c := l.classOf(t.Symbol()); c != nil {
			return hir.Ref(c.Name), true
		}
		if i := l.ifaceOf(t.Symbol()); i != nil {
			return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}, true
		}
	}
	if l.ck.IsArrayType(t) || l.gogenArrayElement(t) != nil {
		for active := range l.mappingTypes {
			if active.Flags()&(checker.TypeFlagsUnion|checker.TypeFlagsIntersection) != 0 || active.Flags()&checker.TypeFlagsObject != 0 && active.Symbol() != nil && !l.librarySymbol(active.Symbol()) {
				return hir.T(hir.Array, hir.T(hir.Dynamic)), true
			}
		}
		return hir.Type{}, false
	}
	if t.Flags()&(checker.TypeFlagsUnion|checker.TypeFlagsIntersection) != 0 || t.ObjectFlags()&checker.ObjectFlagsMapped != 0 {
		return hir.T(hir.Dynamic), true
	}
	if t.Flags()&checker.TypeFlagsObject != 0 && t.Symbol() != nil && t.Symbol().Flags&(ast.SymbolFlagsTypeLiteral|ast.SymbolFlagsObjectLiteral|ast.SymbolFlagsInterface) != 0 && len(l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)) == 0 {
		return hir.T(hir.Dynamic), true
	}
	return hir.Type{}, false
}

func (l *lowerer) gogenCachedType(t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures {
		return hir.Type{}, false
	}
	mapped, ok := l.gogenMappedTypes[t]
	return mapped, ok
}

func (l *lowerer) gogenRememberType(t *checker.Type, mapped *hir.Type) {
	if !l.unprunedFeatures || t == nil || mapped.Kind == hir.Void || t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsUnion) == 0 {
		return
	}
	if l.gogenMappedTypes == nil {
		l.gogenMappedTypes = map[*checker.Type]hir.Type{}
	}
	l.gogenMappedTypes[t] = *mapped
}

// Instantiated mapped arrays can come from another checker in the pool: its
// library target pointer differs, but the resolved numeric index is unchanged.
func (l *lowerer) gogenArrayElement(t *checker.Type) *checker.Type {
	if t == nil || t.Symbol() == nil || !l.librarySymbol(t.Symbol()) || (t.Symbol().Name != "Array" && t.Symbol().Name != "ReadonlyArray") {
		return nil
	}
	if info := l.ck.GetIndexInfoOfType(t, l.ck.GetNumberType()); info != nil {
		return info.ValueType()
	}
	return nil
}

func (l *lowerer) gogenResolvedArray(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	// Canonical arrays already have a type-argument lowering path, including
	// speculative failure handling for recursive contextual types. The numeric
	// index fallback is only needed for arrays from another pooled checker.
	if !l.unprunedFeatures || t.IsTupleType() || l.ck.IsArrayType(t) {
		return hir.Type{}, false
	}
	if element := l.gogenArrayElement(t); element != nil && element.Flags()&checker.TypeFlagsNever == 0 {
		mapped := l.mapCheckerType(n, element)
		if mapped.Kind == hir.Void {
			return mapped, true
		}
		return hir.T(hir.Array, mapped), true
	}
	return hir.Type{}, false
}

func (l *lowerer) gogenStructuralProperty(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil || len(l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)) != 1 || len(l.ck.GetSignaturesOfType(t, checker.SignatureKindConstruct)) != 0 {
		return hir.Type{}, false
	}
	// Builtin Function members are the constructor/prototype ABI, not
	// application data slots. Preserve their existing class-value handling.
	signature := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)[0]
	if declaration := signature.Declaration(); declaration != nil {
		if file := ast.GetSourceFileOfNode(declaration); file != nil && l.prog.prog.IsSourceFileDefaultLibrary(file.Path()) {
			return hir.Type{}, false
		}
	}
	return l.functionType(n, t), true
}

// Union interfaces with no callable slots are object storage views. Recursive
// members may already be tagged records, so they must not be dynRef-unboxed.
func (l *lowerer) gogenObjectUnion(t hir.Type) bool {
	if !l.unprunedFeatures || t.Kind != hir.InterfaceRef {
		return false
	}
	for _, u := range l.unions {
		if u.iface.Name != t.Name || len(u.parts) == 0 || len(u.iface.Methods) != 0 {
			continue
		}
		for _, part := range u.parts {
			if part.Kind != hir.ClassRef && part.Kind != hir.InterfaceRef && part.Kind != hir.UnionRef {
				return false
			}
		}
		return true
	}
	return false
}
