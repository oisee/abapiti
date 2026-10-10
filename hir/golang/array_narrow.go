package golang

import (
	"github.com/oisee/abapiti/hir"
)

// A fresh, unaliased temporary used only by pushes and this one narrowing
// has no observable identity before conversion. Repack its present values;
// absent values remain an explicit failure. General primitive views are refused.
func (b *body) freshOptionalArray(x *hir.Expr) bool {
	if x.X.Kind != hir.Local || x.Type.Kind != hir.Array || x.X.Type.Kind != hir.Array {
		return false
	}
	src := x.X.Type.Args[0]
	if src.Kind != hir.Optional || src.Args[0].IsRef() || !src.Args[0].Equal(x.Type.Args[0]) {
		return false
	}

	var sequence *hir.Expr
	walkStmt(b.m.Body, func(*hir.Stmt) {}, func(v, _ *hir.Expr) {
		if v.Kind == hir.Seq && v.Y == x {
			sequence = v
		}
	})
	if sequence == nil {
		return false
	}
	name := x.X.Name
	declarations := 0
	valid := true
	allowed := map[*hir.Expr]bool{x.X: true}
	declaration := func(s *hir.Stmt) {
		if s.Kind == hir.VarDecl && s.Name == name {
			declarations++
			if s.X == nil || s.X.Kind != hir.New || !s.X.Type.Equal(x.X.Type) || len(s.X.Args) != 0 {
				valid = false
			}
		}
	}
	walkStmt(sequence.Stmt, declaration, func(v, parent *hir.Expr) {
		if v.Stmt != nil {
			walkStmt(v.Stmt, declaration, func(*hir.Expr, *hir.Expr) {})
		}
		if v.Kind == hir.Local && v.Name == name {
			if parent != nil && parent.Kind == hir.RuntimeOp && parent.Op == "array.push" && parent.X == v {
				allowed[v] = true
			} else {
				valid = false
			}
		}
	})
	walkStmt(b.m.Body, func(*hir.Stmt) {}, func(v, _ *hir.Expr) {
		if v.Kind == hir.Local && v.Name == name && !allowed[v] {
			valid = false
		}
	})
	return valid && declarations == 1
}
