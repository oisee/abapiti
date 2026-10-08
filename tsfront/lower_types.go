package tsfront

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/jsnum"
	"math"
)

// Type mapping. Declared annotations are mapped node by node with the checker
// typing each node; checker types are mapped structurally for expressions.
// Both routes must agree; the checker decides, the syntax only selects.
//
// Policy: TypeScript number uses binary64. Integral syntax is not a range
// proof: pinned OSG-JS does not trap i32 overflow, so i32 cannot enforce a trapping contract.

// mapTypeNode maps a declared type annotation to a HIR type.
func (l *lowerer) mapTypeNode(n *ast.Node) hir.Type {
	t := l.ck.GetTypeFromTypeNode(n)
	if t == nil {
		l.diagf(n, "unsupported-type", "no checker type for annotation")
		return hir.T(hir.Void)
	}
	switch n.Kind {
	case ast.KindNumberKeyword:
		l.diagf(n, "note-number-binary64", "number lowered as binary64")
		return hir.T(hir.Number)
	case ast.KindStringKeyword:
		return hir.T(hir.String)
	case ast.KindBooleanKeyword:
		return hir.T(hir.Bool)
	case ast.KindObjectKeyword:
		return hir.Ref(hir.RootObject)
	case ast.KindVoidKeyword:
		return hir.T(hir.Void)
	case ast.KindUndefinedKeyword:
		return hir.T(hir.Optional, hir.T(hir.Dynamic))
	case ast.KindArrayType:
		return hir.T(hir.Array, l.mapTypeNode(n.AsArrayTypeNode().ElementType))
	case ast.KindParenthesizedType:
		return l.mapTypeNode(n.Type())
	case ast.KindTypeOperator:
		// `readonly T[]` parses as a readonly type operator; readonlyness of
		// the elements is not part of the HIR type.
		return l.mapTypeNode(n.Type())
	case ast.KindUnionType:
		optional := false
		var parts []hir.Type
		for _, u := range n.AsUnionTypeNode().Types.Nodes {
			if u.Kind == ast.KindNullKeyword || isNullLiteralType(u) {
				l.diagf(u, "unsupported-null", "null requires a distinct tagged value")
				return hir.T(hir.Void)
			}
			if u.Kind == ast.KindUndefinedKeyword {
				optional = true
				continue
			}
			parts = append(parts, l.mapTypeNode(u))
		}
		return l.union(n, parts, optional)
	case ast.KindNullKeyword:
		l.diagf(n, "unsupported-null", "null requires a distinct tagged value")
		return hir.T(hir.Void)
	case ast.KindLiteralType:
		return l.mapTypeNodeViaChecker(n)
	case ast.KindTupleType:
		return l.tupleType(n)
	case ast.KindTypeLiteral:
		if t := l.recordType(n); t.Kind != hir.Void {
			return t
		}
		return l.mapTypeNodeViaChecker(n)
	case ast.KindFunctionType, ast.KindConstructorType, ast.KindTypeQuery, ast.KindIndexedAccessType,
		ast.KindIntersectionType, ast.KindMappedType,
		ast.KindConditionalType, ast.KindTemplateLiteralType, ast.KindAnyKeyword, ast.KindUnknownKeyword:
		// No syntax mapping: let the checker's resolved type decide.
		return l.mapTypeNodeViaChecker(n)
	case ast.KindTypeReference:
		return l.mapTypeReference(n)
	}
	l.diagf(n, "unsupported-type", "type annotation %s is not lowered", n.Kind.String())
	return hir.T(hir.Void)
}

// mapTypeNodeViaChecker maps through the checker's resolved type, quieting
// the notes (the syntax node has none of its own).
func (l *lowerer) mapTypeNodeViaChecker(n *ast.Node) hir.Type {
	t := l.ck.GetTypeFromTypeNode(n)
	if t == nil {
		l.diagf(n, "unsupported-type", "no checker type for annotation")
		return hir.T(hir.Void)
	}
	before := len(l.diags)
	mapped := l.mapCheckerType(n, t)
	if hasBlocking(l.diags[before:]) {
		msgs := l.diags[before:]
		l.diags = l.diags[:before]
		_ = msgs
	}
	return mapped
}

func (l *lowerer) mapTypeReference(n *ast.Node) hir.Type {
	sym := l.resolve(n.AsTypeReferenceNode().TypeName)
	name := ""
	if sym != nil {
		name = sym.Name
	}
	targs := n.TypeArguments()
	arg := func(i int) hir.Type {
		if len(targs) <= i {
			l.diagf(n, "unsupported-type", "type reference %s needs %d type arguments", name, i+1)
			return hir.T(hir.Void)
		}
		return l.mapTypeNode(targs[i])
	}
	switch name {
	case "Promise":
		if l.librarySymbol(sym) {
			return hir.Ref(l.opaquePromise())
		}
	case "Set":
		return hir.T(hir.OrderedSet, arg(0))
	case "ReadonlySet":
		return hir.T(hir.OrderedSet, arg(0))
	case "Map":
		return hir.T(hir.OrderedMap, arg(0), arg(1))
	case "ReadonlyMap":
		return hir.T(hir.OrderedMap, arg(0), arg(1))
	case "Array", "ReadonlyArray":
		return hir.T(hir.Array, arg(0))
	case "Readonly":
		// readonlyness is not part of the HIR type
		return arg(0)
	case "Record":
		return hir.T(hir.OrderedMap, hir.T(hir.String), arg(1))
	case "RegExp":
		l.diagf(n, "note-regex-type", "RegExp lowered to the RegExp runtime type")
		return hir.T(hir.RegExp)
	case "Error":
		l.diagf(n, "note-builtin-error-ref", "Error lowered to builtin.Error")
		return hir.Ref(l.builtinError())
	}
	if sym != nil && l.numericEnumOf(sym) != nil {
		return hir.T(hir.Number)
	}
	// String enums lower to their string values.
	if sym != nil && l.enumOf(sym) != nil {
		l.diagf(n, "note-enum-string", "enum type %s lowered as string", name)
		return hir.T(hir.String)
	}
	if sym == nil {
		l.diagf(n, "unsupported-type", "unresolved type reference %s", typeRefName(n))
		return hir.T(hir.Void)
	}
	if len(targs) > 0 {
		// Generic instantiations erase to the declaration's constraint.
		l.diagf(n, "note-generic-erased", "type arguments of %s are erased", name)
	}
	if c := l.classOf(sym); c != nil {
		return hir.Ref(c.Name)
	}
	if t, ok := l.indexOnlyInterface(sym); ok {
		l.diagf(n, "note-record-map", "index-signature interface %s lowered as an insertion-ordered map", name)
		return t
	}
	if i := l.ifaceOf(sym); i != nil {
		return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
	}
	beforeAlias := len(l.diags)
	if c := l.synthFromAlias(n, sym); c != nil {
		return hir.Ref(c.Name)
	}
	l.diags = l.diags[:beforeAlias] // the alias may be a non-shape type; the checker decides below
	if c := l.synthOf(sym); c != nil {
		return hir.Ref(c.Name)
	}
	// The type may still carry its alias even when the symbol lookup did not
	// resolve to the alias declaration.
	if t := l.ck.GetTypeFromTypeNode(n); t != nil && t.Alias() != nil && t.Alias().Symbol() != nil {
		before := len(l.diags)
		if c := l.synthFromAlias(n, t.Alias().Symbol()); c != nil {
			return hir.Ref(c.Name)
		}
		l.diags = l.diags[:before]
	}
	// Aliases of arrays, unions and other aliases map through the checker's
	// resolved type (the alias itself is not an object shape).
	{
		before := len(l.diags)
		mapped := l.mapTypeNodeViaChecker(n)
		if mapped.Kind != hir.Void {
			return mapped
		}
		l.diags = l.diags[:before]
	}
	l.diagf(n, "unsupported-type", "type reference %s is not a lowered declaration", name)
	return hir.T(hir.Void)
}

// union collapses the mapped parts; a single remaining part (with or without
// undefined) is the result, anything wider is rejected.
func (l *lowerer) union(n *ast.Node, parts []hir.Type, optional bool) hir.Type {
	var distinct []hir.Type
	for _, p := range parts {
		if p.Kind == hir.Void {
			return hir.T(hir.Void)
		}
		dup := false
		for _, d := range distinct {
			if d.Equal(p) {
				dup = true
				break
			}
		}
		if !dup {
			distinct = append(distinct, p)
		}
	}
	if len(distinct) == 1 {
		if optional {
			return hir.T(hir.Optional, distinct[0])
		}
		return distinct[0]
	}
	// A union of reference types where one constituent accepts all others
	// (a common base) lowers to that base; override signatures stay
	// compatible and subclass arguments still pass.
	for _, d := range distinct {
		if d.Kind != hir.ClassRef && d.Kind != hir.InterfaceRef {
			continue
		}
		dominates := true
		for _, o := range distinct {
			if !o.Equal(d) && !l.acceptsType(d, o) {
				dominates = false
				break
			}
		}
		if dominates {
			l.diagf(n, "note-union-base", "union lowered to the common base %s", d.Name)
			if optional {
				return hir.T(hir.Optional, d)
			}
			return d
		}
	}
	if view, ok := l.unionInterface(n, distinct); ok {
		if optional {
			return hir.T(hir.Optional, view)
		}
		return view
	}
	// Otherwise a union of references lowers to the shared interface: one
	// whose members every constituent carries (TokenNode | ExpressionNode
	// both satisfy INode), the widest one winning. The check is structural
	// over the lowered members, so it works across the checker pool.
	if base, ok := l.commonInterfaceByNames(distinct); ok {
		l.diagf(n, "note-union-interface", "union lowered to the shared interface %s", base.Name)
		if optional {
			return hir.T(hir.Optional, hir.Type{Kind: hir.InterfaceRef, Name: base.Name})
		}
		return hir.Type{Kind: hir.InterfaceRef, Name: base.Name}
	}
	if base, ok := l.commonInterface(n); ok {
		l.diagf(n, "note-union-interface", "union lowered to the shared interface %s", base.Name)
		if optional {
			return hir.T(hir.Optional, hir.Type{Kind: hir.InterfaceRef, Name: base.Name})
		}
		return hir.Type{Kind: hir.InterfaceRef, Name: base.Name}
	}
	// A union of different kinds (string | class value | reference) lowers
	// to the tagged Dynamic box; `typeof` dispatches on it.
	kinds := map[hir.Kind]bool{}
	for _, d := range distinct {
		kinds[d.Kind] = true
	}
	// Unrelated reference types: the object root keeps identity and
	// instanceof; any member access on it is a (loud) diagnostic.
	if len(kinds) > 0 && len(kinds) <= 2 && !kinds[hir.Dynamic] {
		refsOnly := true
		for k := range kinds {
			if k != hir.ClassRef && k != hir.InterfaceRef {
				refsOnly = false
			}
		}
		if refsOnly {
			l.diagf(n, "note-union-root", "union of unrelated references lowered to the object root")
			if optional {
				return hir.T(hir.Optional, hir.Ref(hir.RootObject))
			}
			return hir.Ref(hir.RootObject)
		}
	}
	if len(kinds) > 1 {
		l.diagf(n, "note-dynamic-union", "union of %d kinds lowered as a Dynamic (tagged) value", len(kinds))
		return hir.T(hir.Dynamic)
	}
	if len(kinds) == 1 {
		for k := range kinds {
			if k == hir.ClassValue {
				l.diagf(n, "note-dynamic-union", "union of class values lowered as Dynamic")
				return hir.T(hir.Dynamic)
			}
		}
	}
	l.diagf(n, "unsupported-type", "union type is not lowered")
	return hir.T(hir.Void)
}

// mapCheckerType maps the checker type of an expression (or an inferred
// declaration type) to a HIR type. Narrowed literal types collapse to their
// base; `T | undefined` becomes Optional<T>.
func (l *lowerer) mapCheckerType(n *ast.Node, t *checker.Type) hir.Type {
	if t == nil {
		l.diagf(n, "unsupported-type", "no checker type")
		return hir.T(hir.Void)
	}
	if l.mappingTypes[t] {
		l.diagf(n, "unsupported-type", "recursive checker type has no lowered nominal reference")
		return hir.T(hir.Void)
	}
	if l.mappingTypes == nil {
		l.mappingTypes = map[*checker.Type]bool{}
	}
	l.mappingTypes[t] = true
	defer delete(l.mappingTypes, t)
	if t.IsUnion() {
		optional := false
		var parts []hir.Type
		for _, c := range t.AsUnionOrIntersectionType().Types() {
			if c.Flags()&checker.TypeFlagsNull != 0 {
				l.diagf(n, "unsupported-null", "null requires a distinct tagged value")
				return hir.T(hir.Void)
			}
			if c.Flags()&checker.TypeFlagsUndefined != 0 {
				optional = true
				continue
			}
			if l.isNeverArray(c) && len(t.AsUnionOrIntersectionType().Types()) > 1 {
				// `never[]` (an empty literal) adds nothing to a union.
				continue
			}
			p := l.mapCheckerType(n, c)
			if p.Kind == hir.Void {
				return p
			}
			parts = append(parts, p)
		}
		if len(parts) == 0 {
			l.diagf(n, "unsupported-type", "union of empty array types has no element type")
			return hir.T(hir.Void)
		}
		return l.union(n, parts, optional)
	}
	flags := t.Flags()
	switch {
	case flags&checker.TypeFlagsTypeParameter != 0:
		// Generic parameters erase to their constraint.
		if c := l.ck.GetConstraintOfTypeParameter(t); c != nil {
			before := len(l.diags)
			mapped := l.mapCheckerType(n, c)
			if hasBlocking(l.diags[before:]) {
				l.diags = l.diags[:before]
				return hir.T(hir.Void)
			}
			return mapped
		}
		l.diagf(n, "unsupported-type", "unconstrained type parameter %s", l.ck.TypeToString(t))
		return hir.T(hir.Void)
	case flags&checker.TypeFlagsNumberLiteral != 0 || flags&checker.TypeFlagsNumber != 0:
		if flags&checker.TypeFlagsNumberLiteral != 0 {
			l.checkNumberLiteral(n, t.AsLiteralType().Value())
		}
		return hir.T(hir.Number)
	case flags&checker.TypeFlagsStringLike != 0:
		return hir.T(hir.String)
	case flags&checker.TypeFlagsBooleanLiteral != 0 || flags&checker.TypeFlagsBoolean != 0:
		return hir.T(hir.Bool)
	case flags&checker.TypeFlagsVoid != 0:
		return hir.T(hir.Void)
	case flags&checker.TypeFlagsUndefined != 0:
		// A standalone undefined result still has a value, unlike void.
		// There is no present payload; retain absence using the tagged ABI.
		if l.hint.Kind == hir.Optional && len(l.hint.Args) == 1 {
			return hir.T(hir.Optional, l.hint.Args[0])
		}
		return hir.T(hir.Optional, hir.T(hir.Dynamic))
	case flags&checker.TypeFlagsNull != 0:
		l.diagf(n, "unsupported-type", "null without an optional context")
		return hir.T(hir.Void)
	case flags&checker.TypeFlagsObject != 0:
		if t.IsTupleType() {
			// Tuple elements are the numeric properties of the type; the
			// property list also carries the apparent array members, so
			// iterate while the numeric properties exist.
			var elems []hir.Type
			for i := 0; ; i++ {
				sym := l.ck.GetPropertyOfType(t, itoa(i))
				if sym == nil {
					break
				}
				et := l.mapCheckerType(n, l.ck.GetTypeOfSymbol(sym))
				if et.Kind == hir.Void {
					return hir.T(hir.Void)
				}
				elems = append(elems, et)
			}
			if len(elems) == 0 {
				return hir.T(hir.Void)
			}
			same := true
			for _, e := range elems[1:] {
				if !e.Equal(elems[0]) {
					same = false
					break
				}
			}
			if same {
				// A homogeneous tuple behaves as an array of its element
				// type (map/join/length all work).
				return hir.T(hir.Array, elems[0])
			}
			return l.tupleShape(n, elems)
		}
		// Constructor-signature types (and type aliases of them) are class
		// values: `new () => Expression` and `typeof SomeClass`.
		if len(l.ck.GetSignaturesOfType(t, checker.SignatureKindConstruct)) > 0 {
			l.diagf(n, "note-class-value-type", "constructor type lowered as a class value")
			return hir.T(hir.ClassValue)
		}
		if l.isThunkType(t) {
			l.diagf(n, "note-thunk-type", "parameterless void function type lowered as the closure thunk interface")
			return l.thunkType()
		}
		// Promises carry only an opaque ABI. No fulfillment, await or async
		// body is translated; coverage traps are the only callable producers.
		if t.Symbol() != nil && t.Symbol().Name == "Promise" && l.librarySymbol(t.Symbol()) {
			return hir.Ref(l.opaquePromise())
		}
		// Array and library-collection reference types.
		if l.ck.IsArrayType(t) || (t.Symbol() != nil && (t.Symbol().Name == "Set" || t.Symbol().Name == "Map" || t.Symbol().Name == "ReadonlySet" || t.Symbol().Name == "ReadonlyMap")) {
			var checkArgs [](*checker.Type)
			if tr := t.AsTypeReference(); tr != nil {
				checkArgs = l.ck.GetTypeArguments(t)
			} else if l.ck.IsArrayType(t) {
				// Readonly arrays carry no type reference: the element type
				// is the number index signature's value type.
				if info := l.ck.GetIndexInfoOfType(t, l.ck.GetNumberType()); info != nil {
					checkArgs = append(checkArgs, info.ValueType())
				}
			}
			var args []hir.Type
			ok := true
			for _, a := range checkArgs {
				before := len(l.diags)
				at := l.mapCheckerType(n, a)
				if hasBlocking(l.diags[before:]) || at.Kind == hir.Void {
					l.diags = l.diags[:before]
					ok = false
					break
				}
				args = append(args, at)
			}
			if ok && len(args) != 0 {
				switch {
				case t.Symbol() != nil && (t.Symbol().Name == "Set" || t.Symbol().Name == "ReadonlySet"):
					return hir.T(hir.OrderedSet, args...)
				case t.Symbol() != nil && (t.Symbol().Name == "Map" || t.Symbol().Name == "ReadonlyMap"):
					return hir.T(hir.OrderedMap, args...)
				default:
					return hir.T(hir.Array, args...)
				}
			}
		}
		// String-indexed records are maps.
		if infos := l.ck.GetIndexInfosOfType(t); len(infos) == 1 {
			before := len(l.diags)
			kt := l.mapCheckerType(n, infos[0].KeyType())
			vt := l.mapCheckerType(n, infos[0].ValueType())
			if hasBlocking(l.diags[before:]) {
				l.diags = l.diags[:before]
				return hir.T(hir.Void)
			}
			if kt.Kind == hir.String && vt.Kind != hir.Void {
				l.diagf(n, "note-record-map", "index-signature checker type lowered as an insertion-ordered map")
				return hir.T(hir.OrderedMap, kt, vt)
			}
		}
		if t.Symbol() != nil {
			if t.Symbol().Name == "RegExp" && l.classOf(t.Symbol()) == nil {
				l.diagf(n, "note-regex-type", "RegExp lowered to the RegExp runtime type")
				return hir.T(hir.RegExp)
			}
			if t.Symbol().Name == "Error" && l.classOf(t.Symbol()) == nil {
				return hir.Ref(l.builtinError())
			}
			if t.Symbol().Name == "Object" && t.Symbol().ValueDeclaration == nil {
				return hir.Ref(hir.RootObject)
			}
			if l.numericEnumOf(t.Symbol()) != nil && !t.IsClass() {
				return hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Dynamic))
			}
			if l.enumOf(t.Symbol()) != nil && !t.IsClass() {
				l.diagf(n, "note-enum-namespace", "enum value %s lowered as a namespace object", t.Symbol().Name)
				return hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.String))
			}
			if c := l.classOf(t.Symbol()); c != nil {
				return hir.Ref(c.Name)
			}
			if i := l.ifaceOf(t.Symbol()); i != nil {
				return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
			}
		}
		if alias := t.Alias(); alias != nil && alias.Symbol() != nil {
			if c := l.synthOf(alias.Symbol()); c != nil {
				return hir.Ref(c.Name)
			}
			before := len(l.diags)
			if c := l.synthFromAlias(n, alias.Symbol()); c != nil {
				return hir.Ref(c.Name)
			}
			l.diags = l.diags[:before]
		}
		// Anonymous object shapes (fresh literal types included): synthesize
		// from the members.
		if props := l.ck.GetPropertiesOfType(t); len(props) > 0 {
			if c := l.synthFromProperties(n, props); c != nil {
				return hir.Ref(c.Name)
			}
		}
		l.diagf(n, "unsupported-type", "checker type %s is not a lowered declaration", l.ck.TypeToString(t))
		return hir.T(hir.Void)
	case flags&checker.TypeFlagsNonPrimitive != 0:
		return hir.Ref(hir.RootObject)
	case flags&checker.TypeFlagsAny != 0 || flags&checker.TypeFlagsUnknown != 0:
		l.diagf(n, "note-any-object", "any/unknown lowered as the object root")
		return hir.Ref(hir.RootObject)
	}
	l.diagf(n, "unsupported-type", "checker type %s is not lowered", l.ck.TypeToString(t))
	return hir.T(hir.Void)
}

// synthFromProperties builds a shape class from a list of property symbols.
func (l *lowerer) synthFromProperties(n *ast.Node, props []*ast.Symbol) *hir.Class {
	var fields []hir.Field
	var identity strings.Builder
	for _, p := range props {
		before := len(l.diags)
		pt := l.ck.GetTypeOfSymbol(p)
		var ft hir.Type
		if pt != nil && pt.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 {
			ft = hir.T(hir.Optional, hir.Ref(hir.RootObject))
		} else if l.isNeverArray(pt) {
			// An empty literal's never[]: matches any array field of a
			// reused shape; it never defines a shape of its own.
			ft = hir.T(hir.Array, hir.T(hir.Void))
		} else {
			ft = l.mapCheckerType(n, pt)
		}
		if hasBlocking(l.diags[before:]) || ft.Kind == hir.Void {
			l.diags = l.diags[:before]
			return nil
		}
		if p.Flags&ast.SymbolFlagsOptional != 0 && ft.Kind != hir.Optional {
			ft = hir.T(hir.Optional, ft)
		}
		fields = append(fields, hir.Field{Node: l.node(n), Name: p.Name, Type: ft})
		fmt.Fprintf(&identity, "%q:%s;", p.Name, ft.String())
	}
	// Required and optional reference properties have one physical layout.
	// Reuse the existing shape; the checker narrows required reads at use sites.
	for _, c := range l.out.Classes {
		if (!strings.HasPrefix(c.Name, "shape.") && !l.shapeLike[c.Name]) || len(c.Fields) != len(fields) || c.Ctor == nil {
			continue
		}
		// The checker lists properties in its own order: match by name.
		same := true
		for _, f := range fields {
			var existing *hir.Field
			for k := range c.Fields {
				if c.Fields[k].Name == f.Name {
					existing = &c.Fields[k]
				}
			}
			if existing == nil {
				same = false
				break
			}
			a, b := f.Type, existing.Type
			if a.Kind == hir.Optional && a.Args[0].IsRef() {
				a = a.Args[0]
			}
			if b.Kind == hir.Optional && b.Args[0].IsRef() {
				b = b.Args[0]
			}
			if a.Kind == hir.Array && a.Args[0].Kind == hir.Void && b.Kind == hir.Array {
				a = b
			}
			same = same && a.Equal(b)
		}
		if same {
			for _, p := range props {
				for k := range c.Fields {
					if c.Fields[k].Name == p.Name {
						l.fields[p] = c.Fields[k]
					}
				}
			}
			return c
		}
	}
	key := fmt.Sprintf("shape.%x", sha256.Sum256([]byte(identity.String())))
	for _, c := range l.out.Classes {
		if c.Name == key {
			return c
		}
	}
	// A data object with a subset of a data interface's fields (the rest
	// optional) is that data interface with the others absent: plain
	// objects have no identity of their own.
	for _, c := range l.out.Classes {
		if !l.shapeLike[c.Name] || c.Ctor == nil || len(c.Fields) <= len(fields) {
			continue
		}
		matched := 0
		ok := true
		for _, cf := range c.Fields {
			found := false
			for _, f := range fields {
				if f.Name != cf.Name {
					continue
				}
				a, b := f.Type, cf.Type
				if a.Kind == hir.Optional && a.Args[0].IsRef() {
					a = a.Args[0]
				}
				if b.Kind == hir.Optional && b.Args[0].IsRef() {
					b = b.Args[0]
				}
				if a.Kind == hir.Array && a.Args[0].Kind == hir.Void && (b.Kind == hir.Array || (b.Kind == hir.Optional && b.Args[0].Kind == hir.Array)) {
					a = b
				}
				if !a.Equal(b) && !(b.Kind == hir.Optional && b.Args[0].Equal(a)) {
					ok = false
				}
				found = true
			}
			if found {
				matched++
			} else if cf.Type.Kind != hir.Optional {
				ok = false
			}
		}
		if ok && matched == len(fields) {
			for _, p := range props {
				for _, cf := range c.Fields {
					if cf.Name == p.Name {
						l.fields[p] = cf
					}
				}
			}
			l.diagf(n, "note-shape-subset", "anonymous shape lowered as the data interface %s with absent extra fields", c.Name)
			return c
		}
	}
	for _, f := range fields {
		if f.Type.Kind == hir.Array && f.Type.Args[0].Kind == hir.Void {
			l.diagf(n, "unsupported-type", "object literal with an empty array field %s matches no declared shape (%s)", f.Name, identity.String())
			return nil
		}
	}
	c := &hir.Class{Node: l.node(n), Name: key}
	ctor := &hir.Method{Node: l.node(n), Name: "constructor", Result: hir.T(hir.Void)}
	var list []*hir.Stmt
	for idx, p := range props {
		ft := fields[idx].Type
		c.Fields = append(c.Fields, hir.Field{Node: l.node(n), Name: p.Name, Type: ft})
		// Reference fields share one physical layout with their optional
		// views, so the constructor accepts an absent reference and stores
		// it as the initial reference.
		pt := ft
		value := hir.V(p.Name, ft)
		if ft.IsRef() {
			pt = hir.T(hir.Optional, ft)
			value = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: ft, X: hir.V(p.Name, pt)}
		}
		ctor.Params = append(ctor.Params, hir.Param{Name: p.Name, Type: pt})
		list = append(list, &hir.Stmt{Kind: hir.Assign, Node: l.node(n),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: p.Name, Type: ft,
				X: &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(key)}},
			Y: value})
		l.fields[p] = hir.Field{Node: l.node(n), Name: p.Name, Type: ft}
		if p.Parent != nil {
			l.fieldsBy[p.Parent.Name+"."+p.Name] = hir.Field{Node: l.node(n), Name: p.Name, Type: ft}
		}
	}
	ctor.Body = hir.B(list...)
	c.Ctor = ctor
	l.out.Classes = append(l.out.Classes, c)
	return c
}

// checkNumberLiteral rejects values outside the finite binary64 domain.
func (l *lowerer) checkNumberLiteral(n *ast.Node, v any) {
	f, ok := v.(float64)
	if !ok {
		if j, isNum := v.(jsnum.Number); isNum {
			f, ok = float64(j), true
		}
	}
	if !ok || math.IsInf(f, 0) || math.IsNaN(f) {
		l.diagf(n, "unsupported-number", "literal %v is not finite binary64", v)
	}
}

// synthFromAlias returns (creating it once) the class synthesized for an
// object-shape type alias.
func (l *lowerer) synthFromAlias(n *ast.Node, sym *ast.Symbol) *hir.Class {
	if sym == nil {
		return nil
	}
	if c := l.synthOf(sym); c != nil {
		return c
	}
	decl := sym.ValueDeclaration
	if decl == nil && len(sym.Declarations) > 0 {
		decl = sym.Declarations[0]
	}
	if decl == nil || decl.Kind != ast.KindTypeAliasDeclaration {
		l.diagf(n, "unsupported-type", "alias %s has no lowered declaration", sym.Name)
		return nil
	}
	lit := decl.Type()
	if lit == nil || lit.Kind != ast.KindTypeLiteral {
		l.diagf(n, "unsupported-type", "alias %s is not an object shape", sym.Name)
		return nil
	}
	f := ast.GetSourceFileOfNode(decl)
	// The alias may be declared in another file than the one being lowered;
	// report its members with their own locations.
	savedFile := l.file
	l.file = f
	defer func() { l.file = savedFile }()
	c := &hir.Class{Node: l.node(decl), Name: l.qualifiedName(f, sym.Name)}
	if l.shapeLike == nil {
		l.shapeLike = map[string]bool{}
	}
	l.shapeLike[c.Name] = true
	// Register before the members so recursive shapes resolve to the same
	// class instead of recursing.
	l.synths[sym] = c
	if sf := l.fileOfSymbol(sym); sf != nil {
		l.synthsByName[sf.FileName()+" "+sym.Name] = c
	}
	for _, m := range lit.AsTypeLiteralNode().Members.Nodes {
		if m.Kind != ast.KindPropertySignature {
			l.diagf(m, "unsupported-type", "alias %s has a non-property member", sym.Name)
			return nil
		}
		if m.Name() == nil || m.Name().Kind != ast.KindIdentifier {
			l.diagf(m, "unsupported-type", "alias %s has a computed member name", sym.Name)
			return nil
		}
		var ft hir.Type
		if m.Type() != nil {
			ft = l.mapTypeNode(m.Type())
		} else if ms := m.Symbol(); ms != nil {
			ft = l.mapCheckerType(m, l.ck.GetTypeOfSymbol(ms))
		}
		if m.QuestionToken() != nil && ft.Kind != hir.Optional {
			ft = hir.T(hir.Optional, ft)
		}
		f := hir.Field{Node: l.node(m), Name: m.Name().Text(), Type: ft}
		c.Fields = append(c.Fields, f)
		if ms := m.Symbol(); ms != nil {
			l.fields[ms] = f
		}
	}
	ctor := &hir.Method{Node: l.node(decl), Name: "constructor", Result: hir.T(hir.Void)}
	var list []*hir.Stmt
	for _, f := range c.Fields {
		ctor.Params = append(ctor.Params, hir.Param{Name: f.Name, Type: f.Type})
		list = append(list, &hir.Stmt{Kind: hir.Assign, Node: l.node(decl),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(decl), Name: f.Name, Type: f.Type,
				X: &hir.Expr{Kind: hir.This, Node: l.node(decl), Type: hir.Ref(c.Name)}},
			Y: hir.V(f.Name, f.Type)})
	}
	ctor.Body = hir.B(list...)
	c.Ctor = ctor
	l.out.Classes = append(l.out.Classes, c)
	return c
}

// acceptsType reports whether a value of type src can flow to dst following
// the lowered class hierarchy (mirrors the HIR verifier's rule).
func (l *lowerer) acceptsType(dst, src hir.Type) bool {
	if dst.Equal(src) {
		return true
	}
	if dst.Kind == hir.ClassRef && dst.Name == hir.RootObject && src.IsRef() {
		return true
	}
	if dst.Kind == hir.Optional && len(dst.Args) == 1 {
		if src.Kind == hir.Optional {
			return l.acceptsType(dst.Args[0], src.Args[0])
		}
		return l.acceptsType(dst.Args[0], src)
	}
	if dst.Kind == hir.Array && src.Kind == hir.Array && len(dst.Args) == 1 && len(src.Args) == 1 {
		return l.acceptsType(dst.Args[0], src.Args[0])
	}
	if dst.Kind == hir.InterfaceRef && src.Kind == hir.InterfaceRef {
		var d, s *hir.Interface
		for _, i := range l.out.Interfaces {
			if i.Name == dst.Name {
				d = i
			}
			if i.Name == src.Name {
				s = i
			}
		}
		if d == nil || s == nil {
			return false
		}
		for _, m := range d.Methods {
			var other *hir.Method
			for _, o := range s.Methods {
				if o.Name == m.Name {
					other = o
					break
				}
			}
			if other == nil || !m.Result.Equal(other.Result) || len(m.Params) != len(other.Params) {
				return false
			}
			for j, p := range m.Params {
				if !p.Type.Equal(other.Params[j].Type) || p.Name != other.Params[j].Name {
					return false
				}
			}
		}
		return true
	}
	if src.Kind != hir.ClassRef {
		return false
	}
	for name := src.Name; name != ""; {
		var c *hir.Class
		for _, x := range l.out.Classes {
			if x.Name == name {
				c = x
				break
			}
		}
		if c == nil {
			return false
		}
		if dst.Kind == hir.InterfaceRef && l.covariants[c.Name][dst.Name] {
			return true
		}
		if c.Name == dst.Name && dst.Kind == hir.ClassRef {
			return true
		}
		for _, i := range c.Implements {
			if dst.Kind == hir.InterfaceRef && dst.Name == i {
				return true
			}
		}
		name = c.Super
	}
	return false
}

func typeRefName(n *ast.Node) string {
	t := n.AsTypeReferenceNode().TypeName
	if t == nil {
		return "?"
	}
	return t.Text()
}

// tupleType lowers [A, B, ...] to a synthesized shape class with positional
// fields f0..fn, shared per element-type list.
func (l *lowerer) tupleType(n *ast.Node) hir.Type {
	tt := n.AsTupleTypeNode()
	if tt == nil || tt.Elements == nil {
		return hir.T(hir.Void)
	}
	var elems []hir.Type
	for _, el := range tt.Elements.Nodes {
		et := l.mapTypeNode(el)
		if et.Kind == hir.Void {
			return hir.T(hir.Void)
		}
		elems = append(elems, et)
	}
	if len(elems) > 0 {
		same := true
		for _, e := range elems[1:] {
			if !e.Equal(elems[0]) {
				same = false
				break
			}
		}
		if same {
			return hir.T(hir.Array, elems[0])
		}
	}
	return l.tupleShape(n, elems)
}

// typeSig is a compact signature for shape-class names.
func typeSig(t hir.Type) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(t.String()))) }

// tupleShape returns (creating once) the shape class for an element-type
// list.
func (l *lowerer) tupleShape(n *ast.Node, elems []hir.Type) hir.Type {
	key := "tuple"
	for _, e := range elems {
		key += "." + typeSig(e)
	}
	for _, c := range l.out.Classes {
		if c.Name == key {
			return hir.Ref(c.Name)
		}
	}
	c := &hir.Class{Node: l.node(n), Name: key}
	ctor := &hir.Method{Node: l.node(n), Name: "constructor", Result: hir.T(hir.Void)}
	var list []*hir.Stmt
	for i, e := range elems {
		fn := "f" + itoa(i)
		c.Fields = append(c.Fields, hir.Field{Node: l.node(n), Name: fn, Type: e})
		ctor.Params = append(ctor.Params, hir.Param{Name: fn, Type: e})
		list = append(list, &hir.Stmt{Kind: hir.Assign, Node: l.node(n),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: fn, Type: e,
				X: &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(key)}},
			Y: hir.V(fn, e)})
	}
	ctor.Body = hir.B(list...)
	c.Ctor = ctor
	l.out.Classes = append(l.out.Classes, c)
	return hir.Ref(c.Name)
}

// recordType lowers { [k: string]: T } to OrderedMap<String, T>.
func (l *lowerer) recordType(n *ast.Node) hir.Type {
	lit := n.AsTypeLiteralNode()
	if lit == nil || lit.Members == nil {
		return hir.T(hir.Void)
	}
	for _, m := range lit.Members.Nodes {
		if m.Kind != ast.KindIndexSignature {
			continue
		}
		is := m.AsIndexSignatureDeclaration()
		params := m.Parameters()
		if len(params) != 1 || is.Type == nil {
			return hir.T(hir.Void)
		}
		vt := l.mapTypeNode(is.Type)
		if vt.Kind == hir.Void {
			return hir.T(hir.Void)
		}
		l.diagf(m, "note-record-map", "index-signature record lowered as an insertion-ordered map")
		return hir.T(hir.OrderedMap, hir.T(hir.String), vt)
	}
	return hir.T(hir.Void)
}

// memberNamesOf lists the lowered method names of a class and its chain.
func (l *lowerer) memberNamesOf(name string) map[string]bool {
	out := map[string]bool{}
	for c := l.classByName(name); c != nil; c = l.classByName(c.Super) {
		for _, m := range c.Methods {
			out[m.Name] = true
		}
	}
	return out
}

func (l *lowerer) classByName(name string) *hir.Class {
	for _, c := range l.out.Classes {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// commonInterfaceByNames finds the lowered interface whose member names every
// reference constituent carries; the widest (most methods) wins.
func (l *lowerer) commonInterfaceByNames(parts []hir.Type) (*hir.Interface, bool) {
	sets := make([]map[string]bool, 0, len(parts))
	for _, p := range parts {
		if p.Kind != hir.ClassRef {
			return nil, false
		}
		sets = append(sets, l.memberNamesOf(p.Name))
	}
	var best *hir.Interface
	for i := range l.out.Interfaces {
		iface := l.out.Interfaces[i]
		if len(iface.Methods) == 0 {
			continue
		}
		ok := true
		for _, m := range iface.Methods {
			for _, s := range sets {
				if !s[m.Name] {
					ok = false
				}
			}
		}
		if ok && (best == nil || len(iface.Methods) > len(best.Methods)) {
			best = iface
		}
	}
	return best, best != nil
}

// commonInterface finds the lowered interface every reference constituent of
// the union at n satisfies, preferring the one with the most methods.
func (l *lowerer) commonInterface(n *ast.Node) (*hir.Interface, bool) {
	t := l.ck.GetTypeFromTypeNode(n)
	if t == nil {
		t = l.ck.GetTypeAtLocation(n)
	}
	if t == nil || !t.IsUnion() {
		return nil, false
	}
	var parts []*checker.Type
	for _, c := range t.AsUnionOrIntersectionType().Types() {
		if c.Flags()&checker.TypeFlagsUndefined != 0 || c.Flags()&checker.TypeFlagsNull != 0 {
			continue
		}
		parts = append(parts, c)
	}
	if len(parts) < 2 {
		return nil, false
	}
	_ = parts

	var best *hir.Interface
	for i := range l.out.Interfaces {
		iface := l.out.Interfaces[i]
		if len(iface.Methods) == 0 {
			continue
		}
		decl := l.ifaceNodes[iface.Name]
		if decl == nil || decl.Symbol() == nil {
			continue
		}
		ifaceType := l.ck.GetDeclaredTypeOfSymbol(decl.Symbol())
		if ifaceType == nil {
			continue
		}
		all := true
		for _, p := range parts {
			if !l.ck.IsTypeAssignableTo(p, ifaceType) {
				all = false
				break
			}
		}
		if all && (best == nil || len(iface.Methods) > len(best.Methods)) {
			best = iface
		}
	}
	return best, best != nil
}

// isNullLiteralType matches the `null` literal inside a union type node.
func isNullLiteralType(u *ast.Node) bool {
	if u == nil || u.Kind != ast.KindLiteralType {
		return false
	}
	lt := u.AsLiteralTypeNode()
	return lt != nil && lt.Literal != nil && lt.Literal.Kind == ast.KindNullKeyword
}
