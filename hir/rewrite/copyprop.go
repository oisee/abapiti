package rewrite

import "github.com/oisee/abapiti/hir"

// CopyPropagate removes declared copies of locals. In a statement list,
// `var x: T = y` with y a local of type T is dropped and the later reads of
// x read y instead, when neither x nor y is assigned or declared again in
// the rest of the list (including a Seq's result). Both then hold the same
// value at every read of x: y does not change after the copy, and reading a
// local has no effect. The inliner binds receivers and arguments this way,
// and the frontend binds narrowing guards. It returns the declarations
// removed. Nodes reachable twice in a method are left alone. It rewrites p
// in place.
func CopyPropagate(p *hir.Program) int {
	n := 0
	for _, c := range p.Classes {
		for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if m != nil && m.Body != nil {
				cp := copyProp{uses: map[*hir.Expr]int{}, seqs: map[*hir.Stmt]bool{}}
				cp.count(m.Body, nil)
				cp.stmt(m.Body)
				n += cp.removed
			}
		}
	}
	return n
}

type copyProp struct {
	uses    map[*hir.Expr]int
	seqs    map[*hir.Stmt]bool
	removed int
}

func (c *copyProp) count(s *hir.Stmt, x *hir.Expr) {
	walkCopy(s, x, func(*hir.Stmt) bool { return true }, func(e *hir.Expr) bool {
		c.uses[e]++
		return true
	})
}

// walkCopy visits statements and expressions in s and x until a visitor
// returns false.
func walkCopy(s *hir.Stmt, x *hir.Expr, fs func(*hir.Stmt) bool, fx func(*hir.Expr) bool) bool {
	if s != nil {
		if !fs(s) || !walkCopy(nil, s.X, fs, fx) || !walkCopy(nil, s.Y, fs, fx) || !walkCopy(s.Body, nil, fs, fx) || !walkCopy(s.Else, nil, fs, fx) {
			return false
		}
		for _, t := range s.List {
			if !walkCopy(t, nil, fs, fx) {
				return false
			}
		}
	}
	if x != nil {
		if !fx(x) || !walkCopy(x.Stmt, x.X, fs, fx) || !walkCopy(nil, x.Y, fs, fx) || !walkCopy(nil, x.Z, fs, fx) {
			return false
		}
		for _, a := range x.Args {
			if !walkCopy(nil, a, fs, fx) {
				return false
			}
		}
	}
	return true
}

func (c *copyProp) stmt(s *hir.Stmt) {
	walkCopy(s, nil, func(t *hir.Stmt) bool {
		// A Seq's block is its own scope with the Seq's result (below).
		if t.Kind == hir.Block && !c.seqs[t] {
			t.List = c.list(t.List, nil)
		}
		return true
	}, func(e *hir.Expr) bool {
		if e.Kind == hir.Seq && e.Stmt != nil && e.Stmt.Kind == hir.Block {
			c.seqs[e.Stmt] = true
			e.Stmt.List = c.list(e.Stmt.List, e.Y)
		}
		return true
	})
}

// list rewrites one scope; tail is a Seq's result, evaluated after it.
func (c *copyProp) list(list []*hir.Stmt, tail *hir.Expr) []*hir.Stmt {
	for i := 0; i < len(list); i++ {
		s := list[i]
		if s == nil || s.Kind != hir.VarDecl || s.X == nil || s.X.Kind != hir.Local || s.X.Name == s.Name || !s.X.Type.Equal(s.Type) || c.uses[s.X] != 1 {
			continue
		}
		x, y := s.Name, s.X.Name
		rest := &hir.Stmt{Kind: hir.Block, List: list[i+1:]}
		var reads []*hir.Expr
		ok := walkCopy(rest, tail, func(t *hir.Stmt) bool {
			switch t.Kind {
			case hir.Assign:
				return t.X == nil || t.X.Kind != hir.Local || t.X.Name != x && t.X.Name != y
			case hir.VarDecl, hir.ForEach, hir.Try:
				return t.Name != x && t.Name != y
			}
			return true
		}, func(e *hir.Expr) bool {
			if e.Kind == hir.Local && e.Name == x {
				reads = append(reads, e)
				return c.uses[e] == 1 && e.Type.Equal(s.Type)
			}
			return true
		})
		if !ok {
			continue
		}
		for _, e := range reads {
			e.Name = y
		}
		list = append(list[:i:i], list[i+1:]...)
		i--
		c.removed++
	}
	return list
}
