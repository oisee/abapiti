package rewrite

import (
	"reflect"

	"github.com/oisee/abapiti/hir"
)

// same compares expression meaning, omitting optional identities and locations.
func same(a, b *hir.Expr) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind || !a.Type.Equal(b.Type) || a.Name != b.Name || a.Owner != b.Owner || a.Op != b.Op || !reflect.DeepEqual(a.Value, b.Value) || a.Stmt != nil || b.Stmt != nil || len(a.Args) != len(b.Args) {
		return false
	}
	if !same(a.X, b.X) || !same(a.Y, b.Y) || !same(a.Z, b.Z) {
		return false
	}
	for i := range a.Args {
		if !same(a.Args[i], b.Args[i]) {
			return false
		}
	}
	return true
}
func statements(s *hir.Stmt) []*hir.Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == hir.Block {
		return s.List
	}
	return []*hir.Stmt{s}
}
func runtime(e *hir.Expr, op string) bool { return e != nil && e.Kind == hir.RuntimeOp && e.Op == op }
func staticMap(e *hir.Expr) bool {
	return e != nil && e.Kind == hir.StaticGet && e.Type.Kind == hir.OrderedMap
}
func stableKey(e *hir.Expr) bool {
	return e != nil && (e.Kind == hir.Local || e.Kind == hir.Lit) && !e.Type.IsRef()
}
func (x *extractor) shapes(body *hir.Stmt) {
	ss := statements(body)
	// Deliberately exact: no extra statements, else arm, or alternate store.
	if len(ss) == 2 && ss[0].Kind == hir.If && ss[0].Else == nil && ss[1].Kind == hir.Return {
		cond := ss[0].X
		ret := ss[1].X
		bs := statements(ss[0].Body)
		if cond != nil && cond.Kind == hir.Unary && cond.Op == "!" && runtime(cond.X, "map.has") && staticMap(cond.X.X) && len(cond.X.Args) == 1 && stableKey(cond.X.Args[0]) && runtime(ret, "map.get") && same(ret.X, cond.X.X) && len(ret.Args) == 1 && same(ret.Args[0], cond.X.Args[0]) && len(bs) == 2 && bs[0].Kind == hir.VarDecl && bs[1].Kind == hir.ExprStmt {
			compute, store := bs[0].X, bs[1].X
			if compute != nil && compute.Kind == hir.DirectCall && compute.X == nil && len(compute.Args) == 1 && same(compute.Args[0], cond.X.Args[0]) && runtime(store, "map.set") && same(store.X, ret.X) && len(store.Args) == 2 && same(store.Args[0], ret.Args[0]) && store.Args[1].Kind == hir.Local && store.Args[1].Name == bs[0].Name {
				target := x.resolve(compute.Owner, compute.Name)
				if target != "" {
					x.add("memo_shape", x.method, x.fieldOwner(ret.X.Owner, ret.X.Name), ret.X.Name, target)
				}
			}
		}
	}
	// A method whose outer guard reads a static bool and ends by setting that
	// same flag true. This records lazy initialisation, not thread safety.
	if len(ss) == 1 && ss[0].Kind == hir.If && ss[0].Else == nil {
		cond := ss[0].X
		bs := statements(ss[0].Body)
		if cond != nil && cond.Kind == hir.Unary && cond.Op == "!" && cond.X != nil && cond.X.Kind == hir.StaticGet && cond.X.Type.Kind == hir.Bool && len(bs) > 0 {
			last := bs[len(bs)-1]
			if last.Kind == hir.Assign && same(last.X, cond.X) && last.Y != nil && last.Y.Kind == hir.Lit && last.Y.Value == true {
				x.add("flag_init", x.fieldOwner(cond.X.Owner, cond.X.Name))
			}
		}
	}
}
