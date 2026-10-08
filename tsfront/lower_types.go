package tsfront

import (
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
	case ast.KindVoidKeyword:
		return hir.T(hir.Void)
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
			if u.Kind == ast.KindUndefinedKeyword {
				optional = true
				continue
			}
			parts = append(parts, l.mapTypeNode(u))
		}
		return l.union(n, parts, optional)
	case ast.KindTypeReference:
		return l.mapTypeReference(n)
	}
	l.diagf(n, "unsupported-type", "type annotation %s is not lowered", n.Kind.String())
	return hir.T(hir.Void)
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
	}
	if sym == nil {
		l.diagf(n, "unsupported-type", "unresolved type reference %s", typeRefName(n))
		return hir.T(hir.Void)
	}
	if c := l.classOf(sym); c != nil {
		return hir.Ref(c.Name)
	}
	if i := l.ifaceOf(sym); i != nil {
		return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
	}
	if c := l.synthFromAlias(n, sym); c != nil {
		return hir.Ref(c.Name)
	}
	// The type may still carry its alias even when the symbol lookup did not
	// resolve to the alias declaration.
	if t := l.ck.GetTypeFromTypeNode(n); t != nil && t.Alias() != nil && t.Alias().Symbol() != nil {
		if c := l.synthFromAlias(n, t.Alias().Symbol()); c != nil {
			return hir.Ref(c.Name)
		}
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
	if t.IsUnion() {
		optional := false
		var parts []hir.Type
		for _, c := range t.AsUnionOrIntersectionType().Types() {
			if c.Flags()&checker.TypeFlagsUndefined != 0 {
				optional = true
				continue
			}
			p := l.mapCheckerType(n, c)
			if p.Kind == hir.Void {
				return p
			}
			parts = append(parts, p)
		}
		return l.union(n, parts, optional)
	}
	flags := t.Flags()
	switch {
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
		// `undefined` on its own is only lowered as the absent value of an
		// Optional, with the contextual element type from the hint.
		if l.hint.Kind == hir.Optional && len(l.hint.Args) == 1 {
			return hir.T(hir.Optional, l.hint.Args[0])
		}
		l.diagf(n, "unsupported-type", "undefined without an optional context")
		return hir.T(hir.Void)
	case flags&checker.TypeFlagsObject != 0:
		if t.Symbol() != nil {
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
			l.diagf(n, "unsupported-type", "object type %s is not a lowered shape", alias.Symbol().Name)
			return hir.T(hir.Void)
		}
		l.diagf(n, "unsupported-type", "checker type %s is not a lowered declaration", l.ck.TypeToString(t))
		return hir.T(hir.Void)
	}
	l.diagf(n, "unsupported-type", "checker type %s is not lowered", l.ck.TypeToString(t))
	return hir.T(hir.Void)
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
	l.synths[sym] = c
	if old, ok := l.synthsByName[sym.Name]; ok && old != c {
		l.diagf(n, "unsupported-decl", "duplicate alias name %s", sym.Name)
		return nil
	}
	l.synthsByName[sym.Name] = c
	l.out.Classes = append(l.out.Classes, c)
	return c
}

// acceptsType reports whether a value of type src can flow to dst following
// the lowered class hierarchy (mirrors the HIR verifier's rule).
func (l *lowerer) acceptsType(dst, src hir.Type) bool {
	if dst.Equal(src) {
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
