package golang

import (
	"fmt"
	"math"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Character classifiers are a backend representation of a very narrow proof:
// one straight-line constructor builds a readonly static Number set, publishes
// it once, and every other reference is the receiver of set.has. The builder
// never escapes. Readonly alone does not prove the set is immutable.
type characterSet struct {
	field   *hir.Expr
	publish *hir.Stmt
	allowed map[*hir.Expr]bool
	values  []int
}

func walkExpr(x *hir.Expr, f func(*hir.Expr, *hir.Expr), parent *hir.Expr) {
	if x == nil {
		return
	}
	f(x, parent)
	walkExpr(x.X, f, x)
	walkExpr(x.Y, f, x)
	walkExpr(x.Z, f, x)
	for _, a := range x.Args {
		walkExpr(a, f, x)
	}
	walkStmt(x.Stmt, func(*hir.Stmt) {}, f)
}
func walkStmt(s *hir.Stmt, sf func(*hir.Stmt), ef func(*hir.Expr, *hir.Expr)) {
	if s == nil {
		return
	}
	sf(s)
	// Preserve the assignment context even when HIR nodes are shared.
	walkExpr(s.X, ef, &hir.Expr{Stmt: s})
	walkExpr(s.Y, ef, nil)
	walkStmt(s.Body, sf, ef)
	walkStmt(s.Else, sf, ef)
	for _, v := range s.List {
		walkStmt(v, sf, ef)
	}
}
func staticKey(x *hir.Expr) string { return x.Owner + "\x00" + x.Name }
func constantNumber(x *hir.Expr, constants map[string]float64) (float64, bool) {
	if x == nil {
		return 0, false
	}
	switch x.Kind {
	case hir.NumericConvert:
		if x.Type.Kind == hir.Number && (x.X.Type.Kind == hir.I32 || x.X.Type.Kind == hir.I64) {
			return constantNumber(x.X, constants)
		}
	case hir.StaticGet:
		v, ok := constants[staticKey(x)]
		return v, ok
	case hir.Lit:
		switch v := x.Value.(type) {
		case int:
			return float64(v), true
		case int32:
			return float64(v), true
		case int64:
			return float64(v), true
		case float64:
			return v, true
		}
	}
	return 0, false
}
func (e *emitter) characterSets() {
	e.classifiers = map[string]string{}
	for _, c := range e.p.Classes {
		// An inherited spelling of a static field needs alias normalization.
		// Restrict this proof to leaf owners instead of assuming that spelling.
		if !e.concreteClass(c.Name) {
			continue
		}
		readonly := map[string]bool{}
		for _, f := range c.Fields {
			if f.Static && f.Readonly {
				readonly[f.Name] = true
			}
		}
		for _, m := range c.Methods {
			if m.Name != "class_constructor" || m.Body == nil || m.Body.Kind != hir.Block {
				continue
			}
			constants := map[string]float64{}
			list := m.Body.List
			for i, s := range list {
				if s.Kind == hir.Assign && s.X.Kind == hir.StaticGet && s.X.Owner == c.Name && readonly[s.X.Name] {
					if n, ok := constantNumber(s.Y, constants); ok && e.staticAssignedOnce(s, c.Name) {
						constants[staticKey(s.X)] = n
					}
				}
				if s.Kind != hir.VarDecl || s.Type.Kind != hir.OrderedSet || s.Type.Args[0].Kind != hir.Number || s.X == nil || s.X.Kind != hir.New || len(s.X.Args) != 0 {
					continue
				}
				proof := characterSet{allowed: map[*hir.Expr]bool{}}
				for _, next := range list[i+1:] {
					if next.Kind == hir.ExprStmt && next.X.Kind == hir.RuntimeOp && next.X.Op == "set.add" && next.X.X.Kind == hir.Local && next.X.X.Name == s.Name && len(next.X.Args) == 1 {
						n, ok := constantNumber(next.X.Args[0], constants)
						if !ok || math.Trunc(n) != n || n < -1 || n > 255 {
							break
						}
						proof.allowed[next.X.X] = true
						proof.values = append(proof.values, int(n))
						continue
					}
					if next.Kind == hir.Assign && next.X.Kind == hir.StaticGet && next.X.Owner == c.Name && readonly[next.X.Name] && next.Y.Kind == hir.Local && next.Y.Name == s.Name {
						proof.field = next.X
						proof.publish = next
						proof.allowed[next.Y] = true
					}
					break
				}
				if proof.field == nil || !e.membershipOnly(proof, m, s.Name) {
					continue
				}
				name := e.name("classifier." + staticKey(proof.field))
				e.classifiers[staticKey(proof.field)] = name
				var table [256]bool
				eof := false
				for _, v := range proof.values {
					if v == -1 {
						eof = true
					} else {
						table[v] = true
					}
				}
				entries := []string{}
				for i, v := range table {
					if v {
						entries = append(entries, fmt.Sprintf("%d:true", i))
					}
				}
				e.extra.WriteString(fmt.Sprintf("var %s_table = [256]bool{%s}\n", name, strings.Join(entries, ",")))
				e.extra.WriteString(fmt.Sprintf("func %s(s *orderedSet[float64], n float64) bool {if s==nil {panic(rangeFault{})};if n == -1 {return %t};i:=int32(n);return uint32(i)<256 && n==float64(i) && %s_table[i]}\n", name, eof, name))
			}
		}
	}
}
func (e *emitter) staticAssignedOnce(publish *hir.Stmt, owner string) bool {
	count := 0
	for _, c := range e.p.Classes {
		for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if m == nil {
				continue
			}
			walkStmt(m.Body, func(s *hir.Stmt) {
				if s.Kind == hir.Assign && s.X.Kind == hir.StaticGet && s.X.Owner == owner && s.X.Name == publish.X.Name {
					count++
				}
			}, func(*hir.Expr, *hir.Expr) {})
		}
	}
	return count == 1
}
func (e *emitter) membershipOnly(p characterSet, builder *hir.Method, local string) bool {
	valid := true
	for _, c := range e.p.Classes {
		for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if m == nil {
				continue
			}
			walkStmt(m.Body, func(*hir.Stmt) {}, func(x, parent *hir.Expr) {
				if m == builder && x.Kind == hir.Local && x.Name == local && !p.allowed[x] {
					valid = false
				}
				if x.Kind == hir.StaticGet && staticKey(x) == staticKey(p.field) {
					if parent != nil && parent.Stmt == p.publish {
						return
					}
					if parent == nil || parent.Kind != hir.RuntimeOp || parent.Op != "set.has" || parent.X != x {
						valid = false
					}
				}
			})
		}
	}
	return valid && e.staticAssignedOnce(p.publish, p.field.Owner)
}
