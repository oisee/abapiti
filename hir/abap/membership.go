package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// findConstSets finds static sets that a class initializer fills with integer
// constants and that the program only ever asks has(): no other read, no
// write after the initializer. Such a set never changes and never escapes, so
// has(x) is the comparison chain over its members (the lexer's SPLITS, BUFS
// and AFTER_LITERAL, asked once per character).
func (e *emitter) findConstSets() {
	e.constSets = map[string][]string{}
	for _, c := range e.p.Classes {
		for _, m := range c.Methods {
			if m.Name != "class_constructor" || !m.Static || m.Body == nil {
				continue
			}
			e.constSetsIn(c, m.Body.List)
		}
	}
	if len(e.constSets) == 0 {
		return
	}
	reads, asks := map[string]int{}, map[string]int{}
	var stmt func(*hir.Stmt)
	var expr func(*hir.Expr)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.StaticGet {
			reads[x.Owner+"."+x.Name]++
		}
		if x.Kind == hir.RuntimeOp && x.Op == "set.has" && x.X != nil && x.X.Kind == hir.StaticGet {
			asks[x.X.Owner+"."+x.X.Name]++
		}
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, c := range s.List {
			stmt(c)
		}
	}
	for _, c := range e.p.Classes {
		if c.Ctor != nil {
			stmt(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			stmt(m.Body)
		}
	}
	// The initializer's assignment target is the one read that is not a has().
	for key := range e.constSets {
		if reads[key] != asks[key]+1 {
			delete(e.constSets, key)
		}
	}
}

// constSetsIn matches `var t = new Set; set.add(t, k)...; C.f = t` in one
// initializer, where every k is an integer constant and t is used nowhere else.
func (e *emitter) constSetsIn(c *hir.Class, list []*hir.Stmt) {
	members := map[string][]string{}
	uses := map[string]int{}
	adds := map[string]int{}
	bad := map[string]bool{}
	var count func(*hir.Expr)
	count = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.Local {
			uses[x.Name]++
		}
		count(x.X)
		count(x.Y)
		count(x.Z)
		for _, a := range x.Args {
			count(a)
		}
	}
	type target struct{ field, local string }
	var targets []target
	for _, s := range list {
		count(s.X)
		count(s.Y)
		switch {
		case s.Kind == hir.VarDecl && s.X != nil && s.X.Kind == hir.New && s.Type.Kind == hir.OrderedSet && isIntType(s.Type.Args[0]):
			members[s.Name] = []string{}
		case s.Kind == hir.ExprStmt && s.X != nil && s.X.Kind == hir.RuntimeOp && s.X.Op == "set.add" && s.X.X != nil && s.X.X.Kind == hir.Local:
			name := s.X.X.Name
			if _, ok := members[name]; !ok {
				continue
			}
			adds[name]++
			if len(s.X.Args) != 1 {
				bad[name] = true
				continue
			}
			v, ok := e.intConstant(s.X.Args[0])
			if !ok {
				bad[name] = true
				continue
			}
			members[name] = append(members[name], v)
		case s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.StaticGet && s.X.Owner == c.Name:
			if l := unwrapLocal(s.Y); l != "" {
				targets = append(targets, target{s.X.Name, l})
			}
		}
	}
	for _, t := range targets {
		vs, ok := members[t.local]
		if !ok || bad[t.local] || len(vs) == 0 || uses[t.local] != adds[t.local]+1 {
			continue
		}
		e.constSets[c.Name+"."+t.field] = vs
	}
}

func isIntType(t hir.Type) bool {
	return t.Kind == hir.I32 || t.Kind == hir.I64
}

func unwrapLocal(x *hir.Expr) string {
	for x != nil && (x.Kind == hir.Cast || x.Kind == hir.Narrow) {
		x = x.X
	}
	if x != nil && x.Kind == hir.Local {
		return x.Name
	}
	return ""
}

// intConstant renders an integer literal or an integer CONSTANTS field.
func (e *emitter) intConstant(x *hir.Expr) (string, bool) {
	for x != nil && (x.Kind == hir.NumericConvert || x.Kind == hir.Cast || x.Kind == hir.Narrow) {
		x = x.X
	}
	if x == nil {
		return "", false
	}
	switch x.Kind {
	case hir.Lit:
		if !isIntType(x.Type) {
			return "", false
		}
		v, ok := constantInit(&hir.Class{Fields: []hir.Field{{Name: "v", Type: x.Type, Static: true}}}, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Name: "v"}, Y: x})
		return v, ok
	case hir.StaticGet:
		if !isIntType(x.Type) {
			return "", false
		}
		v, ok := e.staticConstant(x.Owner, x.Name)
		return strings.Trim(v, "'"), ok
	}
	return "", false
}

// constSetHas emits has() on a constant set as a comparison chain; the
// argument is evaluated once and the set itself is never touched.
func (b *body) constSetHas(x *hir.Expr, n string) bool {
	if x.Op != "set.has" || x.X == nil || x.X.Kind != hir.StaticGet || len(x.Args) != 1 {
		return false
	}
	vs, ok := b.e.constSets[x.X.Owner+"."+x.X.Name]
	if !ok {
		return false
	}
	ps, _, _ := hir.RuntimeSignature(x.Op, x.X.Type)
	v := b.temp(ps[0])
	b.line(v + " = " + b.value(x.Args[0], ps[0]) + ".")
	b.line(n + " = xsdbool( " + v + " = " + vs[0])
	for _, k := range vs[1:] {
		b.line("OR " + v + " = " + k)
	}
	b.line(").")
	return true
}
