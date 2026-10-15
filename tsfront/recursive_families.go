package tsfront

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Identity uses normalized declarations and resolved arguments; checker pointers
// are used only for a call-local cycle detector, never as a published cache key.
func (l *lowerer) familySymbolKey(sym *ast.Symbol) string {
	if sym == nil {
		return ""
	}
	if len(sym.Declarations) == 0 {
		return ""
	}
	return l.qualifiedName(ast.GetSourceFileOfNode(sym.Declarations[0]), sym.Name)
}

func (l *lowerer) familyAliasKey(t *checker.Type) string {
	if t == nil || t.Alias() == nil {
		return ""
	}
	name := l.familySymbolKey(t.Alias().Symbol())
	if name == "" {
		return ""
	}
	for _, a := range t.Alias().TypeArguments() {
		name += "[" + l.familyTypeKey(a, true, map[*checker.Type]bool{}) + "]"
	}
	return name
}

func (l *lowerer) familyTypeKey(t *checker.Type, named bool, visiting map[*checker.Type]bool) string {
	return l.familyKeyCached(t, named, visiting) // family_key_memo.go
}

func (l *lowerer) familyTypeKeyRaw(t *checker.Type, named bool, visiting map[*checker.Type]bool) string {
	if t == nil {
		return "missing"
	}
	if named {
		if name := l.familyAliasKey(t); name != "" {
			return "alias:" + name
		}
		if t.Symbol() != nil && t.Symbol().Flags&(ast.SymbolFlagsInterface|ast.SymbolFlagsClass) != 0 {
			name := l.familySymbolKey(t.Symbol())
			for _, a := range l.familyArguments(t) {
				name += "[" + l.familyTypeKey(a, true, visiting) + "]"
			}
			return "decl:" + name
		}
	}
	if visiting[t] {
		return "back:" + l.familySymbolKey(t.Symbol())
	}
	visiting[t] = true
	defer delete(visiting, t)
	if t.IsUnion() || t.Flags()&checker.TypeFlagsIntersection != 0 {
		parts := []string{}
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			parts = append(parts, l.familyTypeKey(p, true, visiting))
		}
		sort.Strings(parts)
		return fmt.Sprintf("%d:%s", t.Flags(), strings.Join(parts, "|"))
	}
	if t.Flags()&checker.TypeFlagsObject != 0 {
		if args := l.familyArguments(t); len(args) > 0 {
			parts := []string{}
			for _, a := range args {
				parts = append(parts, l.familyTypeKey(a, true, visiting))
			}
			name := "array"
			if t.Symbol() != nil {
				name = t.Symbol().Name
			}
			return name + "<" + strings.Join(parts, ",") + ">"
		}
		parts := []string{}
		for _, p := range l.ck.GetPropertiesOfType(t) {
			parts = append(parts, fmt.Sprintf("%q:%t:%t:%s", p.Name, p.Flags&ast.SymbolFlagsOptional != 0, shapePropertyReadonly(p), l.familyTypeKey(l.ck.GetTypeOfPropertyOfType(t, p.Name), true, visiting)))
		}
		sort.Strings(parts)
		return "{" + strings.Join(parts, ";") + "}"
	}
	return l.ck.TypeToString(t)
}

func (l *lowerer) familyTags(t *checker.Type) []string {
	if t == nil {
		return nil
	}
	if t.IsUnion() {
		var tags []string
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			s := l.familyTags(p)
			if len(s) == 0 {
				return nil
			}
			tags = append(tags, s...)
		}
		sort.Strings(tags)
		return tags
	}
	if t.Flags()&checker.TypeFlagsStringLiteral == 0 {
		return nil
	}
	return []string{fmt.Sprint(t.AsLiteralType().Value())}
}

func (l *lowerer) familyDiscriminant(t *checker.Type) string {
	if !t.IsUnion() || l.familyAliasKey(t) == "" {
		return ""
	}
	parts := t.AsUnionOrIntersectionType().Types()
	if len(parts) < 2 {
		return ""
	}
	names := []string{}
	for _, p := range l.ck.GetPropertiesOfType(parts[0]) {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	for _, name := range names {
		ok := true
		for _, part := range parts {
			p := l.ck.GetPropertyOfType(part, name)
			ok = ok && p != nil && p.Flags&ast.SymbolFlagsOptional == 0 && len(l.familyTags(l.ck.GetTypeOfPropertyOfType(part, name))) > 0
		}
		if ok {
			return name
		}
	}
	return ""
}

func (l *lowerer) familyIsRecursive(t *checker.Type, visiting, seen map[*checker.Type]bool) bool {
	if t == nil {
		return false
	}
	if visiting[t] {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t], visiting[t] = true, true
	defer delete(visiting, t)
	if t.IsUnion() || t.Flags()&checker.TypeFlagsIntersection != 0 {
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			if l.familyIsRecursive(p, visiting, seen) {
				return true
			}
		}
	} else if t.Flags()&checker.TypeFlagsObject != 0 {
		if len(l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)) > 0 || len(l.ck.GetSignaturesOfType(t, checker.SignatureKindConstruct)) > 0 {
			return false
		}
		if args := l.familyArguments(t); len(args) > 0 {
			for _, p := range args {
				if l.familyIsRecursive(p, visiting, seen) {
					return true
				}
			}
			return false
		}
		for _, p := range l.ck.GetPropertiesOfType(t) {
			if p.Flags&ast.SymbolFlagsMethod == 0 && l.familyIsRecursive(l.ck.GetTypeOfPropertyOfType(t, p.Name), visiting, seen) {
				return true
			}
		}
	}
	return false
}

type familyRegistration struct {
	l       *lowerer
	unions  map[string]*hir.UnionDecl
	classes map[string]*hir.Class
	shapes  map[string]hir.Type
	checks  map[string]*checker.Type
	failed  bool
	reason  string
}

func (l *lowerer) recursiveFamilyType(n *ast.Node, t *checker.Type) (res hir.Type, handled bool) {
	if !l.unprunedFeatures || t == nil {
		return hir.Type{}, false
	}
	keys := l.familyKeyEnter(n)
	defer keys.exit(&res, &handled)
	l.familyUtilitySources(n, map[*ast.Node]bool{})
	if t.Flags()&checker.TypeFlagsNever != 0 && n.Kind == ast.KindTypeReference && len(l.familyTypes) > 0 {
		if n.Parent != nil && ast.IsFunctionLike(n.Parent) && n.Parent.Type() == n {
			return hir.T(hir.Void), true
		}
		l.diagf(n, "unsupported-type", "empty recursive subset has no payload inhabitants")
		return hir.T(hir.Void), true
	}
	alias := l.familyAliasKey(t)
	if mapped, ok := l.familyTypes[alias]; ok {
		return mapped, true
	}
	if l.familyFailed[alias] {
		l.diagf(n, "unsupported-type", "recursive family %s has an unsupported member", alias)
		return hir.T(hir.Void), true
	}
	if len(l.familyShapes) > 0 && (t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsIntersection) != 0) {
		if mapped, ok := l.familyShapes[l.familyTypeKey(t, false, map[*checker.Type]bool{})]; ok {
			return mapped, true
		}
	}
	if mapped, ok := l.familySubsetType(t); ok {
		return mapped, true
	}
	if alias == "" || l.familyDiscriminant(t) == "" || !l.familyIsRecursive(t, map[*checker.Type]bool{}, map[*checker.Type]bool{}) {
		return hir.Type{}, false
	}
	r := &familyRegistration{l: l, unions: map[string]*hir.UnionDecl{}, classes: map[string]*hir.Class{}, shapes: map[string]hir.Type{}, checks: map[string]*checker.Type{}}
	mapped := r.typ(n, t)
	if l.familyTypes == nil {
		l.familyTypes = map[string]hir.Type{}
		l.familyFailed = map[string]bool{}
		l.familyShapes = map[string]hir.Type{}
		l.familyChecks = map[string]*checker.Type{}
	}
	if r.failed || mapped.Kind == hir.Void || keys.refused() {
		for key := range r.unions {
			l.familyFailed[key] = true
		}
		l.familyFailed[alias] = true
		l.diagf(n, "unsupported-type", "recursive family %s has an unsupported member (%s); component not published", alias, r.reason)
		return hir.T(hir.Void), true
	}
	// Nothing enters public registries until every reachable layout is complete.
	names := []string{}
	for name := range r.classes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c := r.classes[name]
		if existing := l.classByName(name); existing != nil {
			existing.Fields, existing.Ctor, existing.Structural = c.Fields, c.Ctor, true
		} else {
			l.out.Classes = append(l.out.Classes, c)
		}
		if l.shapeLike == nil {
			l.shapeLike = map[string]bool{}
		}
		l.shapeLike[name] = true
	}
	for key, typ := range r.checks {
		l.familyChecks[key] = typ
	}
	for key, typ := range r.shapes {
		l.familyShapes[key] = typ
	}
	names = nil
	for name := range r.unions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		u := r.unions[name]
		u.Complete = true
		l.out.Unions = append(l.out.Unions, u)
		l.familyTypes[name] = hir.Type{Kind: hir.UnionRef, Name: name}
	}
	return mapped, true
}

func (r *familyRegistration) typ(n *ast.Node, t *checker.Type) hir.Type {
	l := r.l
	if t == nil {
		r.failed = true
		r.reason = "missing checker type"
		return hir.T(hir.Void)
	}
	alias := l.familyAliasKey(t)
	if u := r.unions[alias]; u != nil {
		return hir.Type{Kind: hir.UnionRef, Name: u.Name}
	}
	if u, ok := l.familyTypes[alias]; ok {
		return u
	}
	if t.IsUnion() {
		if tag := l.familyDiscriminant(t); tag != "" {
			decl := t.Alias().Symbol().Declarations[0]
			u := &hir.UnionDecl{Node: l.familyNode(decl), Name: alias, Discriminant: tag}
			r.unions[alias] = u
			for _, part := range t.AsUnionOrIntersectionType().Types() {
				key := l.familyTypeKey(part, false, map[*checker.Type]bool{})
				id := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
				payload := r.record(decl, part, "family.payload."+id)
				u.Alternatives = append(u.Alternatives, hir.UnionAlternative{ID: id, Tags: l.familyTags(l.ck.GetTypeOfPropertyOfType(part, tag)), Payload: payload, Literals: l.familyLiterals(part)})
			}
			sort.Slice(u.Alternatives, func(i, j int) bool { return u.Alternatives[i].ID < u.Alternatives[j].ID })
			return hir.Type{Kind: hir.UnionRef, Name: alias}
		}
		optional, nullable := false, false
		var parts []hir.Type
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			if p.Flags()&checker.TypeFlagsUndefined != 0 {
				optional = true
				continue
			}
			if p.Flags()&checker.TypeFlagsNull != 0 {
				nullable = true
				continue
			}
			parts = append(parts, r.typ(n, p))
		}
		if nullable {
			return hir.T(hir.Dynamic)
		} // Existing proven nullable path; typed absence is Slice 4.
		if len(parts) > 0 {
			result := parts[0]
			for _, p := range parts {
				if !result.Equal(p) {
					// Reuse the existing tagged storage for supported mixed scalar
					// and reference unions. This does not establish a typed nullable ABI.
					result = hir.T(hir.Dynamic)
					break
				}
			}
			if optional {
				return hir.T(hir.Optional, result)
			}
			return result
		}
	}
	switch {
	case t.Flags()&checker.TypeFlagsNumberLike != 0:
		return hir.T(hir.Number)
	case t.Flags()&checker.TypeFlagsStringLike != 0:
		return hir.T(hir.String)
	case t.Flags()&checker.TypeFlagsBooleanLike != 0:
		return hir.T(hir.Bool)
	case t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown|checker.TypeFlagsNull) != 0:
		return hir.T(hir.Dynamic)
	case t.Flags()&checker.TypeFlagsNonPrimitive != 0:
		return hir.Ref(hir.RootObject)
	case t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsIntersection) != 0:
		args := l.familyArguments(t)
		if l.ck.IsArrayType(t) && len(args) == 1 {
			return hir.T(hir.Array, r.typ(n, args[0]))
		}
		if sym := t.Symbol(); sym != nil {
			if (sym.Name == "Array" || sym.Name == "ReadonlyArray") && l.librarySymbol(sym) && len(args) == 1 {
				return hir.T(hir.Array, r.typ(n, args[0]))
			}
			if (sym.Name == "Map" || sym.Name == "ReadonlyMap") && l.librarySymbol(sym) && len(args) == 2 {
				return hir.T(hir.OrderedMap, r.typ(n, args[0]), r.typ(n, args[1]))
			}
			if (sym.Name == "Set" || sym.Name == "ReadonlySet") && l.librarySymbol(sym) && len(args) == 1 {
				return hir.T(hir.OrderedSet, r.typ(n, args[0]))
			}
			if c := l.classOf(sym); c != nil && !l.shapeLike[c.Name] {
				data := false
				for _, d := range sym.Declarations {
					data = data || d.Kind == ast.KindInterfaceDeclaration && l.isDataInterface(d)
				}
				if !data {
					return hir.Ref(c.Name)
				}
			}
			if i := l.ifaceOf(sym); i != nil {
				return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
			}
		}
		if infos := l.ck.GetIndexInfosOfType(t); len(infos) == 1 && len(l.ck.GetPropertiesOfType(t)) == 0 {
			return hir.T(hir.OrderedMap, r.typ(n, infos[0].KeyType()), r.typ(n, infos[0].ValueType()))
		}
		name := alias
		if name == "" && t.Symbol() != nil && t.Symbol().Flags&(ast.SymbolFlagsInterface|ast.SymbolFlagsClass) != 0 {
			name = l.familySymbolKey(t.Symbol())
		}
		if name == "" {
			name = fmt.Sprintf("family.shape.%x", sha256.Sum256([]byte(l.familyTypeKey(t, false, map[*checker.Type]bool{}))))
		}
		return r.record(n, t, name)
	}
	r.failed = true
	if r.reason == "" {
		r.reason = "unsupported checker type " + l.ck.TypeToString(t)
	}
	return hir.T(hir.Void)
}

func (l *lowerer) familyNode(n *ast.Node) hir.Node {
	saved := l.file
	if file := ast.GetSourceFileOfNode(n); file != nil {
		l.file = file
	}
	defer func() { l.file = saved }()
	return l.node(n)
}

func (r *familyRegistration) record(n *ast.Node, t *checker.Type, name string) hir.Type {
	l := r.l
	key := l.familyTypeKey(t, false, map[*checker.Type]bool{})
	if typ, ok := r.shapes[key]; ok {
		return typ
	}
	if typ, ok := l.familyShapes[key]; ok {
		return typ
	}
	if len(l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)) > 0 || len(l.ck.GetSignaturesOfType(t, checker.SignatureKindConstruct)) > 0 || len(l.ck.GetIndexInfosOfType(t)) > 0 {
		r.failed = true
		if r.reason == "" {
			r.reason = "callable or indexed layout " + name
		}
		return hir.T(hir.Void)
	}
	ref := hir.Ref(name)
	r.checks[ref.Name] = t
	r.shapes[key] = ref // Private provisional identity, never visible on failure.
	c := &hir.Class{Node: l.familyNode(n), Name: name, Structural: true}
	r.classes[name] = c
	props := append([]*ast.Symbol(nil), l.ck.GetPropertiesOfType(t)...)
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	ctor := &hir.Method{Node: c.Node, Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}
	for _, p := range props {
		for _, d := range p.Declarations {
			if d.Kind == ast.KindGetAccessor || d.Kind == ast.KindSetAccessor || d.ModifierFlags()&(ast.ModifierFlagsPrivate|ast.ModifierFlagsProtected) != 0 {
				r.failed = true
				if r.reason == "" {
					r.reason = "non-data member " + name + "." + p.Name
				}
			}
		}
		if p.Flags&ast.SymbolFlagsMethod != 0 {
			r.failed = true
			if r.reason == "" {
				r.reason = "method-valued member " + name + "." + p.Name
			}
			continue
		}
		ft := r.typ(n, l.ck.GetTypeOfPropertyOfType(t, p.Name))
		if ft.Kind == hir.Void {
			r.failed = true
		}
		if p.Flags&ast.SymbolFlagsOptional != 0 && ft.Kind != hir.Optional {
			ft = hir.T(hir.Optional, ft)
		}
		f := hir.Field{Node: c.Node, Name: p.Name, Type: ft, Readonly: shapePropertyReadonly(p)}
		c.Fields = append(c.Fields, f)
		ctor.Params = append(ctor.Params, hir.Param{Name: f.Name, Type: ft})
		ctor.Body.List = append(ctor.Body.List, &hir.Stmt{Kind: hir.Assign, Node: c.Node, X: &hir.Expr{Kind: hir.FieldGet, Node: c.Node, Name: f.Name, Type: ft, X: &hir.Expr{Kind: hir.This, Node: c.Node, Type: ref}}, Y: hir.V(f.Name, ft)})
	}
	c.Ctor = ctor
	return ref
}

func (l *lowerer) familyArguments(t *checker.Type) []*checker.Type {
	if t != nil && t.ObjectFlags()&checker.ObjectFlagsReference != 0 && t.Target() != nil {
		return l.ck.GetTypeArguments(t)
	}
	return nil
}
