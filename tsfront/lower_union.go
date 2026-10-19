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

type unionView struct {
	iface *hir.Interface
	parts []hir.Type
}

func (l *lowerer) unionInterface(n *ast.Node, parts []hir.Type) (hir.Type, bool) {
	names := []string{}
	for _, p := range parts {
		if p.Kind != hir.ClassRef && p.Kind != hir.InterfaceRef {
			return hir.Type{}, false
		}
		if strings.HasPrefix(p.Name, "shape.") || isTupleShape(p.Name) {
			return hir.Type{}, false
		}
		names = append(names, p.Name)
	}
	if len(names) < 2 {
		return hir.Type{}, false
	}
	sort.Strings(names)
	key := strings.Join(names, "|")
	if l.unions == nil {
		l.unions = map[string]*unionView{}
	}
	view := l.unions[key]
	if view == nil {
		iface := &hir.Interface{Node: l.node(n), Name: fmt.Sprintf("union.%x", sha256.Sum256([]byte(key)))}
		view = &unionView{iface: iface, parts: append([]hir.Type(nil), parts...)}
		l.unions[key] = view
		l.out.Interfaces = append(l.out.Interfaces, iface)
		for _, p := range parts {
			l.recordImplements(p, hir.Type{Kind: hir.InterfaceRef, Name: iface.Name})
		}
		l.completeUnionInterfaces()
		if l.abiReady {
			l.covariantImplements()
			l.eraseGenericOverrides()
			l.completeUnionInterfaces()
		}
	}
	return hir.Type{Kind: hir.InterfaceRef, Name: view.iface.Name}, true
}

func (l *lowerer) unionMethods(t hir.Type) map[string]*hir.Method {
	result := map[string]*hir.Method{}
	if t.Kind == hir.InterfaceRef {
		for _, iface := range l.out.Interfaces {
			if iface.Name == t.Name {
				for _, method := range iface.Methods {
					result[method.Name] = method
				}
				return result
			}
		}
		return result
	}
	for name := t.Name; name != ""; {
		var c *hir.Class
		for _, x := range l.out.Classes {
			if x.Name == name {
				c = x
				break
			}
		}
		if c == nil {
			break
		}
		for _, m := range c.Methods {
			public := true
			for sym, declared := range l.classes {
				if declared.Name != c.Name {
					continue
				}
				for _, d := range sym.Declarations {
					for _, member := range d.Members() {
						name := ""
						if e, ok := l.overrides[member]; ok && e.Method != nil {
							name = e.Method().Name
						} else if n := member.Name(); n != nil && (n.Kind == ast.KindIdentifier || n.Kind == ast.KindPrivateIdentifier || n.Kind == ast.KindStringLiteral) {
							name = n.Text()
						}
						if name == m.Name && member.ModifierFlags()&(ast.ModifierFlagsPrivate|ast.ModifierFlagsProtected) != 0 {
							public = false
						}
					}
				}
			}
			if public && !m.Static && result[m.Name] == nil {
				result[m.Name] = m
			}
		}
		name = c.Super
	}
	return result
}

// Populate only after erased slots exist. The checker decides the common
// public members; HIR supplies their actual erased ABI, never a constituent cast.
func (l *lowerer) completeUnionInterfaces() {
	keys := []string{}
	for key := range l.unions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		view := l.unions[key]
		if l.ifaceClassBaseOf != nil {
			delete(l.ifaceClassBaseOf, view.iface.Name)
			if base := l.commonBrandedClass(view.parts); base != "" {
				l.ifaceClassBaseOf[view.iface.Name] = base
			}
		}
		// ABAP uses nominal interfaces. Every concrete implementation of a
		// constituent interface must implement the synthesized common view.
		for _, part := range view.parts {
			if part.Kind != hir.InterfaceRef {
				continue
			}
			for _, class := range l.out.Classes {
				implements := l.covariants[class.Name][part.Name]
				for _, name := range class.Implements {
					implements = implements || name == part.Name
				}
				if implements {
					l.recordImplements(hir.Ref(class.Name), hir.Type{Kind: hir.InterfaceRef, Name: view.iface.Name})
				}
			}
		}
		view.iface.Methods = nil
		sets := []map[string]*hir.Method{}
		for _, p := range view.parts {
			sets = append(sets, l.unionMethods(p))
		}
		names := []string{}
		for name := range sets[0] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			m := sets[0][name]
			if strings.Contains(name, "_instantiated_") {
				continue
			}
			if m == nil {
				continue
			}
			same := true
			result := m.Result
			params := append([]hir.Param(nil), m.Params...)
			for _, set := range sets[1:] {
				other := set[name]
				if other == nil {
					same = false
					break
				}
				// Results may differ covariantly: the view returns the
				// wider type; a forwarder can return a subtype into it.
				switch {
				case other.Result.Equal(result):
				case l.acceptsType(result, other.Result):
				case l.acceptsType(other.Result, result):
					result = other.Result
				default:
					same = false
				}
				if !same {
					break
				}
				short, long := params, other.Params
				if len(short) > len(long) {
					short, long = long, short
				}
				for j, p := range short {
					// A rest argument wraps call arguments; an array parameter
					// consumes the array itself. Equal storage types do not make
					// these constituent calling conventions interchangeable.
					if p.Variadic != long[j].Variadic {
						same = false
						break
					}
					// A parameter must be accepted by every constituent: keep
					// the narrower type.
					switch {
					case p.Type.Equal(long[j].Type):
					case l.acceptsType(long[j].Type, p.Type):
						long[j] = hir.Param{Name: long[j].Name, Type: p.Type, Variadic: long[j].Variadic}
					case l.acceptsType(p.Type, long[j].Type):
					default:
						same = false
					}
				}
				for _, p := range long[len(short):] {
					if p.Type.Kind != hir.Optional {
						same = false
					}
				}
				params = append([]hir.Param(nil), long...)
			}
			if same {
				l.renameUnionParams(view.parts, name, params)
				cp := *m
				cp.Abstract = false
				cp.Body = nil
				cp.Result = result
				cp.Params = params
				view.iface.Methods = append(view.iface.Methods, &cp)
			}
		}
	}
}

// Keep data membership independently of the method-only reference ABI. A
// nominal or callable alternative prevents structural admission of this union.
func (l *lowerer) structuralUnionMembers(parts []hir.Type) []hir.StructuralMember {
	var members []hir.StructuralMember
	for _, part := range parts {
		c := l.classByName(part.Name)
		if part.Kind != hir.ClassRef || c == nil || !c.Structural || !hir.DataClassView(l.out, c.Name) {
			return nil
		}
		source := l.structuralUnionSource(c.Name)
		if source == nil {
			return nil
		}
		member := hir.StructuralMember{Type: part}
		for base := c; base != nil; base = l.classByName(base.Super) {
			for _, f := range base.Fields {
				if f.Static {
					continue
				}
				prop := l.ck.GetPropertyOfType(source, f.Name)
				if prop == nil {
					return nil
				}
				t := l.ck.GetTypeOfPropertyOfType(source, f.Name)
				field := hir.StructuralField{Name: f.Name, Type: f.Type, Optional: prop.Flags&ast.SymbolFlagsOptional != 0}
				field.Domains = l.structuralDomains(t)
				member.Fields = append(member.Fields, field)
			}
		}
		members = append(members, member)
	}
	return members
}

// This is a shallow source-domain proof, independent of the physical slot.
// Unknown unsupported domains refuse when present rather than becoming any.
func (l *lowerer) structuralDomains(t *checker.Type) []hir.StructuralDomain {
	if t == nil {
		return nil
	}
	if t.IsUnion() {
		var out []hir.StructuralDomain
		seen := map[hir.StructuralDomain]bool{}
		for _, part := range t.AsUnionOrIntersectionType().Types() {
			for _, domain := range l.structuralDomains(part) {
				if !seen[domain] {
					out = append(out, domain)
					seen[domain] = true
				}
			}
		}
		return out
	}
	if t.IsIntersection() {
		for _, part := range t.AsUnionOrIntersectionType().Types() {
			domains := l.structuralDomains(part)
			if len(domains) != 1 || domains[0].Kind != hir.ClassRef {
				return nil
			}
		}
		return []hir.StructuralDomain{{Kind: hir.ClassRef}}
	}
	f := t.Flags()
	domain := hir.StructuralDomain{}
	switch {
	case f&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0:
		domain.Kind = hir.Dynamic
	case f&(checker.TypeFlagsUndefined|checker.TypeFlagsVoid) != 0:
		domain.Kind = "undefined"
	case f&checker.TypeFlagsNull != 0:
		domain.Kind = "null"
	case f&checker.TypeFlagsStringLiteral != 0:
		domain.Kind = hir.String
		domain.Exact = true
		domain.Literal = fmt.Sprint(t.AsLiteralType().Value())
	case f&checker.TypeFlagsNumberLiteral != 0:
		domain.Kind = hir.Number
		domain.Exact = true
		domain.Literal = fmt.Sprint(t.AsLiteralType().Value())
	case f&checker.TypeFlagsBooleanLiteral != 0:
		domain.Kind = hir.Bool
		domain.Exact = true
		domain.Literal = fmt.Sprint(t.AsLiteralType().Value())
	case f&checker.TypeFlagsString != 0:
		domain.Kind = hir.String
	case f&checker.TypeFlagsNumber != 0:
		domain.Kind = hir.Number
	case f&checker.TypeFlagsBoolean != 0:
		domain.Kind = hir.Bool
	case f&checker.TypeFlagsObject != 0:
		if t.IsTupleType() || l.ck.IsArrayType(t) {
			domain.Kind = hir.Array
		} else if len(l.ck.GetSignaturesOfType(t, checker.SignatureKindConstruct)) > 0 {
			domain.Kind = hir.ClassValue
		} else if len(l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)) > 0 {
			return nil
		} else {
			domain.Kind = hir.ClassRef
		}
	default:
		return nil
	}
	return []hir.StructuralDomain{domain}
}

func (l *lowerer) completeStructuralUnionMembers() {
	for _, view := range l.unions {
		view.iface.StructuralMembers = l.structuralUnionMembers(view.parts)
	}
}

// A common property shape is a live view of the union's original record,
// A constituent projection additionally checks that member's source domains.
func (l *lowerer) structuralUnionProjection(src, dst hir.Type) bool {
	if src.Kind != hir.InterfaceRef || dst.Kind != hir.ClassRef || !l.shapeLike[dst.Name] {
		return false
	}
	if view := l.structuralReferenceUnion(src); view != nil {
		for _, part := range view.parts {
			if part.Equal(dst) {
				return false
			}
		}
		return true
	}
	return false
}

func (l *lowerer) structuralReferenceUnion(t hir.Type) *unionView {
	if !l.unprunedFeatures || t.Kind != hir.InterfaceRef {
		return nil
	}
	for _, view := range l.unions {
		if view.iface.Name != t.Name {
			continue
		}
		for _, part := range view.parts {
			if part.Kind != hir.ClassRef || !l.shapeLike[part.Name] || !hir.DataClassView(l.out, part.Name) || l.structuralUnionSource(part.Name) == nil {
				return nil
			}
		}
		return view
	}
	return nil
}

// Reused physical layouts are not source-domain proofs. Anonymous family
// views can share a layout without having a declaration for every slot; keep
// their existing native boundary instead of emitting an unprovable projection.
func (l *lowerer) structuralUnionSource(name string) *checker.Type {
	source := l.structuralUnionSources[name]
	if source == nil {
		for sym, candidate := range l.classes {
			if candidate.Name == name {
				source = l.ck.GetDeclaredTypeOfSymbol(sym)
				break
			}
		}
	}
	if source == nil {
		for sym, candidate := range l.synths {
			if candidate.Name == name {
				source = l.ck.GetDeclaredTypeOfSymbol(sym)
				break
			}
		}
	}
	if source == nil {
		return nil
	}
	for c := l.classByName(name); c != nil; c = l.classByName(c.Super) {
		for _, field := range c.Fields {
			if !field.Static && l.ck.GetPropertyOfType(source, field.Name) == nil {
				return nil
			}
		}
	}
	if l.structuralUnionSources == nil {
		l.structuralUnionSources = map[string]*checker.Type{}
	}
	l.structuralUnionSources[name] = source
	return source
}

func (l *lowerer) checkedStructuralUnion(x *hir.Expr) *hir.Expr {
	if l.structuralReferenceUnion(x.Type) != nil {
		return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), x.Type)
	}
	return x
}

// Structural public members alone do not prove a native class identity. Every
// constituent must carry the same private/protected instance class brand.
func (l *lowerer) commonBrandedClass(parts []hir.Type) string {
	if len(parts) == 0 {
		return ""
	}
	baseName := func(part hir.Type) string {
		if part.Kind == hir.ClassRef {
			return part.Name
		}
		if part.Kind == hir.InterfaceRef {
			return l.ifaceClassBaseOf[part.Name]
		}
		return ""
	}
	for candidate := l.classByName(baseName(parts[0])); candidate != nil; candidate = l.classByName(candidate.Super) {
		if !l.classHasNominalBrand(candidate.Name) {
			continue
		}
		common := true
		for _, part := range parts[1:] {
			found := false
			for class := l.classByName(baseName(part)); class != nil; class = l.classByName(class.Super) {
				if class.Name == candidate.Name {
					found = true
					break
				}
			}
			common = common && found
		}
		if common {
			return candidate.Name
		}
	}
	return ""
}
