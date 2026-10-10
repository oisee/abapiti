package golang

import "github.com/oisee/abapiti/hir"

// Plan only a newly reset private array on this receiver, immediately consumed
// by an internal method with a string input. Capacity is a hint, never a bound;
// the array keeps its identity and ordinary growth handles larger results.
func (b *body) capacityPlans(block *hir.Stmt) map[*hir.Expr][]*hir.Expr {
	plans := map[*hir.Expr][]*hir.Expr{}
	fresh := []*hir.Expr{}
	for _, s := range block.List {
		if s == nil {
			continue
		}
		if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.FieldGet && s.X.X != nil && s.X.X.Kind == hir.This {
			c := b.e.fieldOwner(b.c, s.X.Name)
			if c == nil {
				fresh = nil
				continue
			}
			private := false
			for _, f := range c.Fields {
				if f.Name == s.X.Name {
					private = f.Private
				}
			}
			if private && s.X.Type.Kind == hir.Array && s.Y.Kind == hir.New && len(s.Y.Args) == 0 && b.arrayAppended(s.X.Name) {
				fresh = append(fresh, s.X)
			} else if s.X.Type.IsRef() {
				fresh = nil
			}
			continue
		}
		if s.Kind == hir.ExprStmt && s.X.Kind == hir.VirtualCall && s.X.X != nil && s.X.X.Kind == hir.This {
			m, _ := b.e.method(b.c, s.X.Name)
			if m != nil && m.Internal {
				for _, p := range m.Params {
					if p.Type.Kind == hir.String {
						plans[s.X] = fresh
						break
					}
				}
			}
		}
		fresh = nil
	}
	return plans
}

func (b *body) arrayAppended(name string) bool {
	found := false
	for _, m := range b.c.Methods {
		walkStmt(m.Body, func(*hir.Stmt) {}, func(x, _ *hir.Expr) {
			if x.Kind == hir.RuntimeOp && x.Op == "array.push" && x.X.Kind == hir.FieldGet && x.X.X.Kind == hir.This && x.X.Name == name {
				found = true
			}
		})
	}
	return found
}
