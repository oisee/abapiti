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
		if p.Kind != hir.ClassRef {
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
			for _, set := range sets[1:] {
				other := set[name]
				if other == nil || !other.Result.Equal(m.Result) {
					same = false
					break
				}
				short, long := m, other
				if len(short.Params) > len(long.Params) {
					short, long = long, short
				}
				for j, p := range short.Params {
					if !p.Type.Equal(long.Params[j].Type) {
						same = false
					}
				}
				for _, p := range long.Params[len(short.Params):] {
					if p.Type.Kind != hir.Optional {
						same = false
					}
				}
				m = long
			}
			if same {
				cp := *m
				cp.Abstract = false
				cp.Body = nil
				cp.Params = append([]hir.Param(nil), m.Params...)
				view.iface.Methods = append(view.iface.Methods, &cp)
			}
		}
	}
}
