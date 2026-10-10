package rewrite

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

type inlineTemplate struct {
	value    *hir.Expr
	body     []*hir.Stmt
	literals map[string]bool
}
type inlineAction struct {
	r        *runner
	prefix   string
	serial   int
	cache    map[*hir.Method]*inlineTemplate
	prepared map[*hir.Method]bool
}

func newInlineAction(r *runner) *inlineAction {
	a := &inlineAction{r: r, prefix: "inl_", cache: map[*hir.Method]*inlineTemplate{}, prepared: map[*hir.Method]bool{}}
	for {
		used := false
		check := func(n string) {
			if strings.HasPrefix(n, a.prefix) {
				used = true
			}
		}
		for m := range r.methods {
			for _, p := range m.Params {
				check(p.Name)
			}
			visitTree(m.Body, "", func(s *hir.Stmt, _ string) {
				if s.Kind == hir.VarDecl || s.Kind == hir.ForEach || s.Kind == hir.Try {
					check(s.Name)
				}
			}, func(e *hir.Expr, _ string) {
				if e.Kind == hir.Local {
					check(e.Name)
				}
			})
		}
		if !used {
			return a
		}
		a.prefix = "x" + a.prefix
	}
}
func (a *inlineAction) expand(call *hir.Expr, statement *hir.Stmt, caller *hir.Method, depth int) (*hir.Expr, *hir.Stmt) {
	if call.Kind != hir.VirtualCall || call.X == nil || call.X.Type.Kind != hir.ClassRef {
		return nil, nil
	}
	var callee *hir.Method
	for _, row := range a.r.db.Lookup("dispatch", call.X.Type.Name, call.Name) {
		callee = a.r.byID[row[2]]
		break
	}
	if callee == nil || len(call.Args) != len(callee.Params) {
		return nil, nil
	}
	for _, p := range callee.Params {
		if p.Variadic {
			return nil, nil
		}
	}
	if !a.prepared[callee] {
		a.prepared[callee] = true
		a.r.method(callee, depth+1)
		t, ok := makeTemplate(callee)
		if ok {
			a.cache[callee] = t
		}
	}
	t := a.cache[callee]
	if t == nil {
		return nil, nil
	}
	if call.Type.Kind == hir.Void && (statement == nil || t.body == nil) || call.Type.Kind != hir.Void && t.value == nil {
		return nil, nil
	}
	owner := strings.SplitN(a.r.methods[callee], "::", 2)[0]
	label := owner + "." + callee.Name
	c := &inlineCopy{prefix: fmt.Sprintf("%s%d_", a.prefix, a.serial+1), node: hir.Node{ID: call.ID, Source: strings.TrimSpace(call.Source + " inlined " + label)}, used: map[string]bool{}}
	var pre []*hir.Stmt
	selfType := hir.Ref(owner)
	if call.X.Kind == hir.This && call.X.Type.Equal(selfType) {
		c.self = &hir.Expr{Node: call.X.Node, Kind: hir.This, Type: selfType}
	} else {
		name := c.fresh("self")
		pre = append(pre, &hir.Stmt{Node: call.Node, Kind: hir.VarDecl, Name: name, Type: selfType, X: call.X})
		c.self = hir.V(name, selfType)
	}
	env := map[string]*hir.Expr{}
	for i, p := range callee.Params {
		arg := call.Args[i]
		if arg.Kind == hir.Lit && t.literals[p.Name] && arg.Type.Equal(p.Type) {
			env[p.Name] = arg
			continue
		}
		name := c.fresh(p.Name)
		pre = append(pre, &hir.Stmt{Node: call.Node, Kind: hir.VarDecl, Name: name, Type: p.Type, X: arg})
		env[p.Name] = hir.V(name, p.Type)
	}
	var value *hir.Expr
	var body *hir.Stmt
	if t.value != nil {
		value = c.expr(t.value, env)
		if len(pre) > 0 {
			value = &hir.Expr{Node: c.node, Kind: hir.Seq, Type: call.Type, Stmt: &hir.Stmt{Node: c.node, Kind: hir.Block, List: pre}, Y: value}
		}
	} else {
		for _, s := range t.body {
			pre = append(pre, c.stmt(s, env))
		}
		body = &hir.Stmt{Node: c.node, Kind: hir.Block, List: pre}
	}
	old := treeSize(nil, call)
	if statement != nil {
		old = treeSize(statement, nil)
	}
	growth := treeSize(body, value) - old
	if growth < 0 {
		growth = 0
	}
	if a.r.growth[caller]+growth > a.r.limits.MethodGrowth || a.r.total+growth > a.r.limits.ProgramGrowth {
		return nil, nil
	}
	a.r.growth[caller] += growth
	a.r.total += growth
	a.serial++
	a.r.stats.CallSites++
	a.r.stats.Callees[label]++
	return value, body
}
func treeSize(s *hir.Stmt, e *hir.Expr) int {
	n := 0
	fs := func(*hir.Stmt, string) { n++ }
	fx := func(e *hir.Expr, _ string) {
		n++
		// Structural site traversal visits Seq contents but omits their enclosing
		// block. Expansion budgets count that block as a statement too.
		if e.Stmt != nil {
			n++
		}
	}
	visitTree(s, "", fs, fx)
	visitExpr(e, "", fs, fx)
	return n
}

type inlineCopy struct {
	prefix string
	node   hir.Node
	self   *hir.Expr
	used   map[string]bool
}

func (c *inlineCopy) fresh(base string) string {
	name := c.prefix + base
	out := name
	for n := 2; c.used[out]; n++ {
		out = fmt.Sprintf("%s_%d", name, n)
	}
	c.used[out] = true
	return out
}
func copyEnv(e map[string]*hir.Expr) map[string]*hir.Expr {
	r := map[string]*hir.Expr{}
	for k, v := range e {
		r[k] = v
	}
	return r
}
func (c *inlineCopy) expr(e *hir.Expr, env map[string]*hir.Expr) *hir.Expr {
	if e == nil {
		return nil
	}
	if e.Kind == hir.This {
		v := *c.self
		v.Node = c.node
		return &v
	}
	if e.Kind == hir.Local {
		if v := env[e.Name]; v != nil {
			return c.expr(v, nil)
		}
	}
	v := *e
	v.Node = c.node
	if e.Range != nil {
		rg := *e.Range
		v.Range = &rg
	}
	if e.Args != nil {
		v.Args = make([]*hir.Expr, len(e.Args))
		for i, arg := range e.Args {
			v.Args[i] = c.expr(arg, env)
		}
	}
	if e.Kind == hir.Seq {
		inner := copyEnv(env)
		v.Stmt = c.block(e.Stmt, inner)
		v.Y = c.expr(e.Y, inner)
	} else {
		v.Stmt = c.stmt(e.Stmt, env)
		v.Y = c.expr(e.Y, env)
	}
	v.X = c.expr(e.X, env)
	v.Z = c.expr(e.Z, env)
	return &v
}
func (c *inlineCopy) block(s *hir.Stmt, env map[string]*hir.Expr) *hir.Stmt {
	if s == nil {
		return nil
	}
	v := *s
	v.Node = c.node
	v.List = make([]*hir.Stmt, len(s.List))
	for i, x := range s.List {
		v.List[i] = c.stmt(x, env)
	}
	return &v
}
func (c *inlineCopy) stmt(s *hir.Stmt, env map[string]*hir.Expr) *hir.Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == hir.Block {
		return c.block(s, copyEnv(env))
	}
	v := *s
	v.Node = c.node
	v.X = c.expr(s.X, env)
	v.Y = c.expr(s.Y, env)
	v.Body = c.stmt(s.Body, copyEnv(env))
	v.Else = c.stmt(s.Else, copyEnv(env))
	if s.List != nil {
		v.List = make([]*hir.Stmt, len(s.List))
		inner := copyEnv(env)
		for i, x := range s.List {
			v.List[i] = c.stmt(x, inner)
		}
	}
	if s.Kind == hir.VarDecl {
		v.Name = c.fresh(s.Name)
		env[s.Name] = hir.V(v.Name, s.Type)
	}
	return &v
}

// Return lowering is an action primitive. Guards and policy stay in Grace;
// this routine constructs a scoped expression or return-free void body.
type returnLowering struct {
	result       hir.Type
	declarations map[string]int
}

func makeTemplate(m *hir.Method) (*inlineTemplate, bool) {
	if m.Body == nil || m.Body.Kind != hir.Block {
		return nil, false
	}
	t := &inlineTemplate{literals: map[string]bool{}}
	b := returnLowering{m.Result, map[string]int{}}
	for _, p := range m.Params {
		b.declarations[p.Name]++
		t.literals[p.Name] = true
	}
	visitTree(m.Body, "", func(s *hir.Stmt, _ string) {
		if s.Kind == hir.VarDecl {
			b.declarations[s.Name]++
		}
		if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local {
			t.literals[s.X.Name] = false
		}
	}, func(*hir.Expr, string) {})
	value, body, ok := b.lower(m.Body.List)
	t.value, t.body = value, body
	return t, ok
}
func statementList(s *hir.Stmt) []*hir.Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == hir.Block {
		return s.List
	}
	return []*hir.Stmt{s}
}
func returnShape(s *hir.Stmt) (any, all bool) {
	if s == nil {
		return false, false
	}
	visitTree(s, "", func(x *hir.Stmt, _ string) {
		if x.Kind == hir.Return {
			any = true
		}
	}, func(*hir.Expr, string) {})
	switch s.Kind {
	case hir.Return:
		all = true
	case hir.Block:
		for _, v := range s.List {
			_, a := returnShape(v)
			if a {
				all = true
				break
			}
		}
	case hir.If:
		_, y := returnShape(s.Body)
		_, z := returnShape(s.Else)
		all = y && z
	}
	return
}
func (b returnLowering) flat(s *hir.Stmt) bool {
	for _, v := range statementList(s) {
		if v == nil {
			return false
		}
		if v.Kind == hir.VarDecl && b.declarations[v.Name] != 1 {
			return false
		}
	}
	return true
}
func followed(a, z []*hir.Stmt) []*hir.Stmt { v := append([]*hir.Stmt{}, a...); return append(v, z...) }
func (b returnLowering) resultExpr(pre []*hir.Stmt, e *hir.Expr) *hir.Expr {
	if len(pre) == 0 {
		return e
	}
	return &hir.Expr{Node: e.Node, Kind: hir.Seq, Type: b.result, Stmt: hir.B(pre...), Y: e}
}

// lower handles value and void templates through the same structural algorithm.
func (b returnLowering) lower(list []*hir.Stmt) (*hir.Expr, []*hir.Stmt, bool) {
	var pre []*hir.Stmt
	void := b.result.Kind == hir.Void
	for i, s := range list {
		if s == nil {
			return nil, nil, false
		}
		rest := list[i+1:]
		switch s.Kind {
		case hir.Return:
			if void {
				if s.X != nil {
					if s.X.Type.Kind != hir.Void {
						return nil, nil, false
					}
					pre = append(pre, &hir.Stmt{Node: s.Node, Kind: hir.ExprStmt, X: s.X})
				}
				if pre == nil {
					pre = []*hir.Stmt{}
				}
				return nil, pre, true
			}
			if s.X == nil || !s.X.Type.Equal(b.result) {
				return nil, nil, false
			}
			return b.resultExpr(pre, s.X), nil, true
		case hir.VarDecl, hir.Assign, hir.ExprStmt:
			pre = append(pre, s)
		case hir.Block:
			if !b.flat(s) {
				return nil, nil, false
			}
			value, body, ok := b.lower(followed(s.List, rest))
			if !ok {
				return nil, nil, false
			}
			if void {
				return nil, followed(pre, body), true
			}
			return b.resultExpr(pre, value), nil, true
		case hir.If:
			hasY, allY := returnShape(s.Body)
			hasZ, allZ := returnShape(s.Else)
			if !hasY && !hasZ {
				pre = append(pre, s)
				continue
			}
			ys, zs := statementList(s.Body), statementList(s.Else)
			if allY {
				if !b.flat(s.Else) {
					return nil, nil, false
				}
				zs = followed(zs, rest)
			} else if allZ {
				if !b.flat(s.Body) {
					return nil, nil, false
				}
				ys = followed(ys, rest)
			} else {
				return nil, nil, false
			}
			y, yb, yok := b.lower(ys)
			z, zb, zok := b.lower(zs)
			if !yok || !zok {
				return nil, nil, false
			}
			if void {
				return nil, append(pre, &hir.Stmt{Node: s.Node, Kind: hir.If, X: s.X, Body: hir.B(yb...), Else: hir.B(zb...)}), true
			}
			return b.resultExpr(pre, &hir.Expr{Node: s.Node, Kind: hir.Conditional, Type: b.result, X: s.X, Y: y, Z: z}), nil, true
		default:
			return nil, nil, false
		}
	}
	if void {
		if pre == nil {
			pre = []*hir.Stmt{}
		}
		return nil, pre, true
	}
	return nil, nil, false
}
