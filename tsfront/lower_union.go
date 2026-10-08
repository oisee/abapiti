package tsfront

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
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
		if strings.HasPrefix(p.Name, "shape.") || strings.HasPrefix(p.Name, "tuple.") {
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
