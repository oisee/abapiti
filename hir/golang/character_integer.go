package golang

import (
	"github.com/oisee/abapiti/hir"
	"math"
)

// Prove that Number values are exact int32 values without changing their HIR
// types. All definitions of a local must qualify; Number parameters and cycles
// fail closed. Only leaf-receiver methods may contribute a return summary.
func (e *emitter) characterInteger(x *hir.Expr, m *hir.Method, visiting map[*hir.Expr]bool, methods map[*hir.Method]bool) bool {
	if x == nil {
		return false
	}
	if x.Type.Kind == hir.I32 {
		return true
	}
	if x.Type.Kind != hir.Number || visiting[x] {
		return false
	}
	visiting[x] = true
	defer delete(visiting, x)
	switch x.Kind {
	case hir.Lit:
		n, ok := constantNumber(x, nil)
		return ok && math.Trunc(n) == n && n >= math.MinInt32 && n <= math.MaxInt32
	case hir.NumericConvert:
		return x.X.Type.Kind == hir.I32
	case hir.RuntimeOp:
		return x.Op == "number.fromI32"
	case hir.Local:
		for _, p := range m.Params {
			if p.Name == x.Name {
				return false
			}
		}
		defined, valid := false, true
		walkStmt(m.Body, func(s *hir.Stmt) {
			if s.Kind == hir.VarDecl && s.Name == x.Name {
				defined = true
				if s.X != nil && !e.characterInteger(s.X, m, visiting, methods) {
					valid = false
				}
			}
			if s.Kind == hir.Assign && s.X.Kind == hir.Local && s.X.Name == x.Name {
				defined = true
				if !e.characterInteger(s.Y, m, visiting, methods) {
					valid = false
				}
			}
			if s.Kind == hir.ForEach && s.Name == x.Name {
				valid = false
			}
			if s.Kind == hir.Try && s.Name == x.Name {
				valid = false
			}
		}, func(*hir.Expr, *hir.Expr) {})
		return defined && valid
	case hir.DirectCall, hir.VirtualCall:
		owner := x.Owner
		if x.X != nil {
			owner = x.X.Type.Name
			if !e.concreteClass(owner) {
				return false
			}
		}
		c := e.classBy(owner)
		if c == nil {
			return false
		}
		callee, _ := e.method(c, x.Name)
		if callee == nil || methods[callee] {
			return false
		}
		methods[callee] = true
		defer delete(methods, callee)
		returned, valid := false, true
		walkStmt(callee.Body, func(s *hir.Stmt) {
			if s.Kind == hir.Return {
				returned = true
				if !e.characterInteger(s.X, callee, visiting, methods) {
					valid = false
				}
			}
		}, func(*hir.Expr, *hir.Expr) {})
		return returned && valid
	}
	return false
}
