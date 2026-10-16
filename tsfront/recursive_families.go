package tsfront

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
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
		name += "[" + l.familyTypeKeyIndependent(a) + "]"
	}
	return name
}

// Alias arguments are independent keys; ids are context-free (family_key_memo.go).
func (l *lowerer) familyTypeKeyIndependent(t *checker.Type) string {
	return l.familyKeyCached(t, true)
}

// familyTypeKey returns the context-free id of (t, named). The visiting map is
// kept for callers' signatures only; SCC ids need no traversal context.
func (l *lowerer) familyTypeKey(t *checker.Type, named bool, _ map[*checker.Type]bool) string {
	return l.familyKeyCached(t, named) // family_key_memo.go
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

// familyFailure names the root whose component registration failed, so later
// references to any member family can repeat the original reason.
type familyFailure struct{ root, reason string }

type familyRegistration struct {
	l       *lowerer
	unions  map[string]*hir.UnionDecl
	classes map[string]*hir.Class
	shapes  map[string]hir.Type
	checks  map[string]*checker.Type
	failed  bool
	reason  string
	// siblings are the union members around the layout being recorded; an
	// exactly-undefined property takes the field type its siblings declare.
	siblings []*checker.Type
	// member is the declaration whose type is being laid out, for reasons.
	member     string
	memberNode *ast.Node
	// memberOf maps each member of a family being registered to its alias.
	memberOf map[*checker.Type]string
}

func (r *familyRegistration) hasNull(t *checker.Type) bool {
	for _, p := range t.AsUnionOrIntersectionType().Types() {
		if p.Flags()&checker.TypeFlagsNull != 0 {
			return true
		}
	}
	return false
}

// familyMembers maps T in `T | undefined` back to its family when the members
// are exactly a family under registration, or a published family (subset).
// This matches how ordinary lowering types the same annotation, so stored and
// read values share one HIR type.
func (r *familyRegistration) familyMembers(parts []*checker.Type) (hir.Type, bool) {
	if len(parts) < 2 {
		return hir.Type{}, false
	}
	alias := r.memberOf[parts[0]]
	if alias != "" {
		for _, p := range parts {
			if r.memberOf[p] != alias {
				return hir.Type{}, false
			}
		}
		count := 0
		for _, a := range r.memberOf {
			if a == alias {
				count++
			}
		}
		if count == len(parts) {
			return hir.Type{Kind: hir.UnionRef, Name: alias}, true
		}
		return hir.Type{}, false
	}
	return r.l.familySubsetParts(parts)
}

// memberReason names the member being laid out, its location and its type.
func (r *familyRegistration) memberReason(t *checker.Type) string {
	if r.member == "" {
		return ""
	}
	text := r.l.ck.TypeToString(t)
	if len(text) > 200 {
		text = text[:197] + "..."
	}
	loc := ""
	if r.memberNode != nil {
		if file := ast.GetSourceFileOfNode(r.memberNode); file != nil {
			loc = " at " + r.l.prog.locString(file, scanner.GetTokenPosOfNode(r.memberNode, file, false))
		}
	}
	return " for member " + r.member + loc + ": " + text
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
	if failure, ok := l.familyFailed[alias]; ok {
		if failure.root == alias {
			l.diagf(n, "unsupported-type", "recursive family %s has an unsupported member (%s)", alias, failure.reason)
		} else {
			l.diagf(n, "unsupported-type", "recursive family %s has an unsupported member (member of failed component of %s: %s)", alias, failure.root, failure.reason)
		}
		return hir.T(hir.Void), true
	}
	if len(l.familyShapes) > 0 && (t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsIntersection) != 0) && !l.ck.TypeHasCallOrConstructSignatures(t) {
		// A constructor or function type is never a data record, even when
		// its (empty) property key matches a family shape.
		if mapped, ok := l.familyShapes[l.familyTypeKey(t, false, map[*checker.Type]bool{})]; ok {
			return mapped, true
		}
	}
	if mapped, ok := l.familySubsetType(t); ok {
		return mapped, true
	}
	if mapped, ok := l.familyCopyMember(t); ok {
		return mapped, true
	}
	if alias == "" || l.familyDiscriminant(t) == "" || !l.familyIsRecursive(t, map[*checker.Type]bool{}, map[*checker.Type]bool{}) {
		return hir.Type{}, false
	}
	r := &familyRegistration{l: l, unions: map[string]*hir.UnionDecl{}, classes: map[string]*hir.Class{}, shapes: map[string]hir.Type{}, checks: map[string]*checker.Type{}}
	mapped := r.typ(n, t)
	if l.familyTypes == nil {
		l.familyTypes = map[string]hir.Type{}
		l.familyFailed = map[string]familyFailure{}
		l.familyShapes = map[string]hir.Type{}
		l.familyChecks = map[string]*checker.Type{}
	}
	if r.failed || mapped.Kind == hir.Void || keys.refused() {
		failure := familyFailure{root: alias, reason: r.reason}
		for key := range r.unions {
			l.familyFailed[key] = failure
		}
		l.familyFailed[alias] = failure
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
			if r.memberOf == nil {
				r.memberOf = map[*checker.Type]string{}
			}
			for _, part := range t.AsUnionOrIntersectionType().Types() {
				r.memberOf[part] = alias
			}
			for _, part := range t.AsUnionOrIntersectionType().Types() {
				key := l.familyTypeKey(part, false, map[*checker.Type]bool{})
				id := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
				r.siblings = t.AsUnionOrIntersectionType().Types()
				payload := r.record(decl, part, "family.payload."+id)
				u.Alternatives = append(u.Alternatives, hir.UnionAlternative{ID: id, Tags: l.familyTags(l.ck.GetTypeOfPropertyOfType(part, tag)), Payload: payload, Literals: l.familyLiterals(part)})
			}
			sort.Slice(u.Alternatives, func(i, j int) bool { return u.Alternatives[i].ID < u.Alternatives[j].ID })
			return hir.Type{Kind: hir.UnionRef, Name: alias}
		}
		optional, nullable := false, false
		var parts []hir.Type
		var present []*checker.Type
		members := l.familyLiveParts(t.AsUnionOrIntersectionType().Types())
		for _, p := range members {
			if p.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsNull) == 0 {
				present = append(present, p)
			}
		}
		if optional := len(present) < len(members); optional {
			if family, ok := r.familyMembers(present); ok && t.Flags()&checker.TypeFlagsUnion != 0 && !r.hasNull(t) {
				return hir.T(hir.Optional, family)
			}
		} else if len(members) < len(t.AsUnionOrIntersectionType().Types()) {
			// An intersection distributed over a family (Type & Type, or
			// Extract<Type, ...> & Type) keeps only live members.
			if len(present) == 1 {
				return r.typ(n, present[0])
			}
			if family, ok := r.familyMembers(present); ok {
				return family
			}
		}
		if len(present) > 1 && !r.hasNull(t) {
			// A homomorphic mapped copy of a family (RuntimeFields<Type>
			// re-creates every member) is that family: same members, checked
			// property by property, not a new record per copy.
			if family, ok := r.familyCopy(present); ok {
				if len(present) < len(members) {
					return hir.T(hir.Optional, family)
				}
				return family
			}
		}
		for _, p := range members {
			if p.Flags()&checker.TypeFlagsUndefined != 0 {
				optional = true
				continue
			}
			if p.Flags()&checker.TypeFlagsNull != 0 {
				nullable = true
				continue
			}
			r.siblings = t.AsUnionOrIntersectionType().Types()
			parts = append(parts, r.typ(n, p))
			r.siblings = nil
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
	case t.Flags()&checker.TypeFlagsBigIntLike != 0:
		return hir.T(hir.BigInt)
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
		if t.IsTupleType() {
			return r.tuple(n, t)
		}
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
		r.reason = "unsupported checker type " + l.ck.TypeToString(t) + r.memberReason(t)
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
	siblings := r.siblings
	r.siblings = nil
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
			r.reason = "callable or indexed layout " + name + r.memberReason(t)
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
		pt := l.ck.GetTypeOfPropertyOfType(t, p.Name)
		savedMember, savedNode := r.member, r.memberNode
		r.member, r.memberNode = name+"."+p.Name, nil
		if len(p.Declarations) > 0 {
			r.memberNode = p.Declarations[0]
		}
		var ft hir.Type
		if pt != nil && pt.Flags()&checker.TypeFlagsUndefined != 0 {
			ft = r.undefinedField(n, t, siblings, p.Name)
		} else {
			ft = r.typ(n, pt)
		}
		r.member, r.memberNode = savedMember, savedNode
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

// tuple lays out a fixed tuple of required elements of one type as an array
// of that type, like non-recursive tuple lowering (lower_types.go). Optional,
// rest and heterogeneous elements have no family layout and refuse.
func (r *familyRegistration) tuple(n *ast.Node, t *checker.Type) hir.Type {
	l := r.l
	refuse := func(why string) hir.Type {
		r.failed = true
		if r.reason == "" {
			r.reason = why + r.memberReason(t)
		}
		return hir.T(hir.Void)
	}
	flags := t.Target().AsTupleType().ElementFlags()
	if len(flags) == 0 {
		return refuse("empty tuple layout")
	}
	elem := hir.Type{}
	for i, f := range flags {
		if f != checker.ElementFlagsRequired {
			return refuse("tuple layout with optional or rest elements")
		}
		sym := l.ck.GetPropertyOfType(t, itoa(i))
		if sym == nil {
			return refuse("tuple layout without element " + itoa(i))
		}
		et := r.typ(n, l.ck.GetTypeOfSymbol(sym))
		if et.Kind == hir.Void {
			return et
		}
		if elem.Kind == "" {
			elem = et
		} else if !elem.Equal(et) {
			return refuse("heterogeneous tuple layout")
		}
	}
	return hir.T(hir.Array, elem)
}

// undefinedField lays out a property whose checker type is exactly undefined
// (x?: undefined or x: undefined) as an absent optional. The payload type is
// the one sibling members declare for the same key, so union reads share one
// field type; key presence stays with the instance's own keys, never the value.
func (r *familyRegistration) undefinedField(n *ast.Node, self *checker.Type, siblings []*checker.Type, name string) hir.Type {
	l := r.l
	inner := hir.Type{}
	for _, s := range siblings {
		if s == self || l.ck.GetPropertyOfType(s, name) == nil {
			continue
		}
		st := l.ck.GetTypeOfPropertyOfType(s, name)
		if st == nil || st.Flags()&checker.TypeFlagsUndefined != 0 {
			continue
		}
		typ := r.typ(n, st)
		if typ.Kind == hir.Optional && len(typ.Args) == 1 {
			typ = typ.Args[0]
		}
		if typ.Kind == hir.Void {
			continue // The sibling's own layout reports the failure.
		}
		if inner.Kind == "" {
			inner = typ
		} else if !inner.Equal(typ) {
			inner = hir.T(hir.Dynamic)
		}
	}
	if inner.Kind == "" {
		inner = hir.T(hir.Dynamic)
	}
	return hir.T(hir.Optional, inner)
}

func (l *lowerer) familyArguments(t *checker.Type) []*checker.Type {
	if t != nil && t.ObjectFlags()&checker.ObjectFlagsReference != 0 && t.Target() != nil {
		return l.ck.GetTypeArguments(t)
	}
	return nil
}

// familyCopy maps union parts that are structural copies of every member of
// one registered (or in-progress) family to that family. A copy has the
// member's discriminant tags, property names and optionality, and each
// property type is mutually assignable with the member's.
func (r *familyRegistration) familyCopy(parts []*checker.Type) (hir.Type, bool) {
	l := r.l
	type family struct {
		name, tag string
		members   []*checker.Type
	}
	var families []family
	byAlias := map[string][]*checker.Type{}
	for member, alias := range r.memberOf {
		byAlias[alias] = append(byAlias[alias], member)
	}
	for alias, members := range byAlias {
		if u := r.unions[alias]; u != nil {
			families = append(families, family{alias, u.Discriminant, members})
		}
	}
	for _, u := range l.out.Unions {
		if !u.Complete || strings.HasPrefix(u.Name, "family.subset.") {
			continue
		}
		var members []*checker.Type
		for _, a := range u.Alternatives {
			if c := l.familyChecks[a.Payload.Name]; c != nil {
				members = append(members, c)
			}
		}
		if len(members) == len(u.Alternatives) {
			families = append(families, family{u.Name, u.Discriminant, members})
		}
	}
	sort.Slice(families, func(i, j int) bool { return families[i].name < families[j].name })
	for _, f := range families {
		if len(f.members) != len(parts) {
			continue
		}
		used := map[*checker.Type]bool{}
		complete := true
		for _, part := range parts {
			tags := strings.Join(l.familyTags(l.ck.GetTypeOfPropertyOfType(part, f.tag)), ",")
			found := false
			for _, m := range f.members {
				if used[m] || tags == "" || tags != strings.Join(l.familyTags(l.ck.GetTypeOfPropertyOfType(m, f.tag)), ",") {
					continue
				}
				if l.familyCopyShape(part, m) {
					used[m], found = true, true
					break
				}
			}
			if !found {
				complete = false
				break
			}
		}
		if complete {
			return hir.Type{Kind: hir.UnionRef, Name: f.name}, true
		}
	}
	return hir.Type{}, false
}

func (l *lowerer) familyCopyShape(part, member *checker.Type) bool {
	a, b := l.ck.GetPropertiesOfType(part), l.ck.GetPropertiesOfType(member)
	if len(a) != len(b) {
		return false
	}
	for _, p := range b {
		q := l.ck.GetPropertyOfType(part, p.Name)
		if q == nil || (q.Flags&ast.SymbolFlagsOptional != 0) != (p.Flags&ast.SymbolFlagsOptional != 0) {
			return false
		}
		x, y := l.ck.GetTypeOfPropertyOfType(part, p.Name), l.ck.GetTypeOfPropertyOfType(member, p.Name)
		if x != y && (!l.ck.IsTypeAssignableTo(x, y) || !l.ck.IsTypeAssignableTo(y, x)) {
			return false
		}
	}
	return true
}

// familyCopyMember maps a structural copy of one published family member (a
// flow-narrowed RuntimeFields<Type> member) to that member's payload.
func (l *lowerer) familyCopyMember(t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil || t.IsUnion() || t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsIntersection) == 0 || l.ck.TypeHasCallOrConstructSignatures(t) {
		return hir.Type{}, false
	}
	for _, u := range l.out.Unions {
		if !u.Complete || strings.HasPrefix(u.Name, "family.subset.") {
			continue
		}
		tags := strings.Join(l.familyTags(l.ck.GetTypeOfPropertyOfType(t, u.Discriminant)), ",")
		if tags == "" {
			continue
		}
		for _, a := range u.Alternatives {
			c := l.familyChecks[a.Payload.Name]
			if c != nil && c != t && tags == strings.Join(a.Tags, ",") && l.familyCopyShape(t, c) {
				return a.Payload, true
			}
		}
	}
	return hir.Type{}, false
}
