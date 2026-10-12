package tsfront

import (
	"sort"

	"github.com/oisee/abapiti/hir"
)

// A nonescaping assertion observed solely through method calls retains the
// actual object's dispatch. The certified union supplies existing methods;
// it never substitutes the asserted class's implementation on a sibling.
func (l *lowerer) preserveSiblingMethodViews() {
	for _, c := range append([]*hir.Class(nil), l.out.Classes...) {
		for _, m := range numberMethods(c) {
			decls := map[string][]*hir.Stmt{}
			viewWalk(m.Body, func(s *hir.Stmt) {
				if s.Kind == hir.VarDecl {
					decls[s.Name] = append(decls[s.Name], s)
				}
			}, nil)
			for name, ds := range decls {
				if len(ds) != 1 {
					continue
				}
				decl := ds[0]
				cast := decl.X
				for cast != nil && cast.Kind == hir.Seq {
					cast = cast.Y
				}
				if cast == nil || cast.Kind != hir.Cast || cast.X == nil || cast.Type.Kind != hir.ClassRef || cast.X.Type.Kind != hir.InterfaceRef || !cast.Type.Equal(decl.Type) {
					continue
				}
				target := l.classByName(cast.Type.Name)
				if target == nil || target.Super == "" {
					continue
				}
				used := map[string]bool{}
				safe := true
				for _, p := range m.Params {
					safe = safe && p.Name != name
				}
				viewWalk(m.Body, func(s *hir.Stmt) {
					if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && s.X.Name == name {
						safe = false
					}
				}, func(x, parent *hir.Expr) {
					if x.Kind == hir.Local && x.Name == name {
						if parent == nil || parent.Kind != hir.VirtualCall || parent.X != x {
							safe = false
						} else {
							used[parent.Name] = true
						}
					}
				})
				if !safe || len(used) == 0 {
					continue
				}
				methods := l.unionMethods(hir.Ref(target.Name))
				var parts []hir.Type
				for _, sibling := range l.out.Classes {
					if sibling.Abstract || !l.acceptsType(hir.Ref(target.Super), hir.Ref(sibling.Name)) {
						continue
					}
					other := l.unionMethods(hir.Ref(sibling.Name))
					compatible := true
					for method := range used {
						compatible = compatible && l.compatibleMethodView(methods[method], other[method])
					}
					if compatible {
						parts = append(parts, hir.Ref(sibling.Name))
					}
				}
				sort.Slice(parts, func(i, j int) bool { return parts[i].Name < parts[j].Name })
				view, ok := l.unionInterface(nil, parts)
				if !ok {
					continue
				}
				*cast = *l.rtOp("dynamic.asRef", l.coerce(cast.X, hir.T(hir.Dynamic)), view)
				decl.Type = view
				for seq := decl.X; seq != nil && seq.Kind == hir.Seq; seq = seq.Y {
					seq.Type = view
				}
				viewWalk(m.Body, nil, func(x, _ *hir.Expr) {
					if x.Kind == hir.Local && x.Name == name {
						x.Type = view
					}
				})
			}
		}
	}
}

func (l *lowerer) compatibleMethodView(a, b *hir.Method) bool {
	if a == nil || b == nil || !a.Result.Equal(b.Result) {
		return false
	}
	short, long := a.Params, b.Params
	if len(short) > len(long) {
		short, long = long, short
	}
	for i, p := range short {
		if !p.Type.Equal(long[i].Type) || p.Variadic || long[i].Variadic {
			return false
		}
	}
	for _, p := range long[len(short):] {
		if p.Type.Kind != hir.Optional || p.Variadic {
			return false
		}
	}
	return true
}
