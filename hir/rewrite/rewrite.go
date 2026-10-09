package rewrite

import (
	"fmt"
	"strconv"

	"github.com/oisee/abapiti/hir"
)

type rewriteRule struct {
	name            string
	priority, bound int
	match           atom
	where           []atom
	action          atom
}

func parseRewrite(n sexpr) (rewriteRule, error) {
	r := rewriteRule{bound: -1}
	if len(n.list) < 6 {
		return r, fmt.Errorf("grace needs name, priority, match, where and action")
	}
	r.name = n.list[1].text
	var err error
	r.priority, err = strconv.Atoi(n.list[2].text)
	if err != nil {
		return r, err
	}
	seen := map[string]bool{}
	for _, s := range n.list[3:] {
		if len(s.list) == 0 {
			return r, fmt.Errorf("empty grace section")
		}
		k := s.list[0].text
		if seen[k] {
			return r, fmt.Errorf("duplicate grace %s", k)
		}
		seen[k] = true
		switch k {
		case "match", "action":
			if len(s.list) != 2 {
				return r, fmt.Errorf("%s needs one atom", k)
			}
			a, e := parseAtom(s.list[1])
			if e != nil {
				return r, e
			}
			if a.negative {
				return r, fmt.Errorf("negative %s", k)
			}
			if k == "match" {
				r.match = a
			} else {
				r.action = a
			}
		case "where":
			for _, v := range s.list[1:] {
				a, e := parseAtom(v)
				if e != nil {
					return r, e
				}
				r.where = append(r.where, a)
			}
		case "bound":
			if len(s.list) != 3 || s.list[1].text != "depth" {
				return r, fmt.Errorf("expected bound depth N")
			}
			r.bound, err = strconv.Atoi(s.list[2].text)
			if err != nil || r.bound < 0 {
				return r, fmt.Errorf("invalid rewrite depth")
			}
		default:
			return r, fmt.Errorf("unknown grace section %s", k)
		}
	}
	if !seen["match"] || !seen["where"] || !seen["action"] {
		return r, fmt.Errorf("grace needs match, where and action")
	}
	if r.match.pred != "node" || len(r.match.args) != 2 {
		return r, fmt.Errorf("match expects (node site kind)")
	}
	if (r.action.pred != "inline" || len(r.action.args) != 1) && (r.action.pred != "replace" || len(r.action.args) != 2) {
		return r, fmt.Errorf("unknown rewrite action")
	}
	bound := map[string]bool{}
	for _, a := range append([]atom{r.match}, r.where...) {
		if !a.negative && !comparison(a.pred) {
			for _, t := range a.args {
				if t.variable {
					bound[t.value] = true
				}
			}
		}
	}
	for _, a := range append([]atom{r.action}, r.where...) {
		if a.negative || comparison(a.pred) || a.pred == r.action.pred {
			for _, t := range a.args {
				if t.wild || t.variable && !bound[t.value] {
					return r, fmt.Errorf("%s: unbound action/guard", r.name)
				}
			}
		}
	}
	if r.action.args[0] != r.match.args[0] {
		return r, fmt.Errorf("action must rewrite the matched node")
	}
	return r, nil
}

// Limits bound rewrite rounds and growth, measured in statements plus expressions.
// Zero values select conservative defaults. Depth bounds in rules cap callee chains.
type Limits struct{ Rounds, MethodGrowth, ProgramGrowth int }
type Stats struct {
	CallSites int
	Callees   map[string]int
	Rounds    int
}
type rewriteNode struct {
	id     string
	expr   *hir.Expr
	stmt   *hir.Stmt
	method *hir.Method
}
type runner struct {
	p              *hir.Program
	db             *DB
	rules          *Rules
	limits         Limits
	stats          Stats
	nodes          map[string]rewriteNode
	ids            map[*hir.Expr]string
	methods        map[*hir.Method]string
	byID           map[string]*hir.Method
	done           map[*hir.Method]bool
	growth         map[*hir.Method]int
	total, changed int
	inline         *inlineAction
	err            error
}

// Rewrite applies rules deterministically, in operand order and callee-first for
// inline actions. A round visits only its input nodes. Analysis is discarded and
// rebuilt after mutations; no derived table survives a rewrite round.
func Rewrite(p *hir.Program, rules *Rules, limits Limits) (Stats, error) {
	r := &runner{p: p, rules: rules, limits: limits, stats: Stats{Callees: map[string]int{}}, growth: map[*hir.Method]int{}}
	if limits.Rounds < 0 || limits.MethodGrowth < 0 || limits.ProgramGrowth < 0 {
		return r.stats, fmt.Errorf("negative rewrite limit")
	}
	if r.limits.Rounds == 0 {
		r.limits.Rounds = 1
	}
	if r.limits.MethodGrowth == 0 {
		r.limits.MethodGrowth = 100000
	}
	if r.limits.ProgramGrowth == 0 {
		r.limits.ProgramGrowth = 1000000
	}
	if es := hir.Verify(p); len(es) > 0 {
		return r.stats, fmt.Errorf("invalid HIR: %v", es)
	}
	for round := 0; round < r.limits.Rounds; round++ {
		var err error
		r.db, err = Analyze(p)
		if err != nil {
			return r.stats, err
		}
		r.index()
		if err := r.addInlineFacts(); err != nil {
			return r.stats, err
		}
		if err := Evaluate(r.db, rules); err != nil {
			return r.stats, err
		}
		if err := r.validate(); err != nil {
			return r.stats, err
		}
		r.done = map[*hir.Method]bool{}
		r.changed = 0
		r.inline = newInlineAction(r)
		for _, c := range p.Classes {
			if c.Ctor != nil {
				r.method(c.Ctor, 0)
			}
			for _, m := range c.Methods {
				r.method(m, 0)
			}
		}
		if r.err != nil {
			return r.stats, r.err
		}
		if es := hir.Verify(p); len(es) > 0 {
			return r.stats, fmt.Errorf("rewrite round %d: %v", round+1, es)
		}
		r.stats.Rounds++
		r.db = nil // invalidate all regions and their transitive dependents
		if r.changed == 0 {
			break
		}
	}
	return r.stats, nil
}
func (r *runner) validate() error {
	for _, rr := range r.rules.rewrites {
		for _, a := range append([]atom{rr.match}, rr.where...) {
			if comparison(a.pred) && len(a.args) != 2 {
				return fmt.Errorf("%s needs two terms", a.pred)
			}
			if t := r.db.tables[a.pred]; t != nil && len(t.index) != len(a.args) {
				return fmt.Errorf("%s: inconsistent arity", a.pred)
			}
		}
	}
	return nil
}
func (r *runner) index() {
	r.nodes = map[string]rewriteNode{}
	r.ids = map[*hir.Expr]string{}
	r.methods = map[*hir.Method]string{}
	r.byID = map[string]*hir.Method{}
	for _, c := range r.p.Classes {
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			id := methodID(c.Name, name)
			r.methods[m] = id
			r.byID[id] = m
			visitTree(m.Body, id+"/body", func(s *hir.Stmt, path string) {
				r.nodes[path] = rewriteNode{path, nil, s, m}
				r.db.Add("node", path, string(s.Kind))
			}, func(e *hir.Expr, path string) {
				r.nodes[path] = rewriteNode{path, e, nil, m}
				r.ids[e] = path
				r.db.Add("node", path, string(e.Kind))
			})
		}
	}
}

// visitTree uses the fact extractor's site paths, including Seq statement paths.
func visitTree(s *hir.Stmt, path string, fs func(*hir.Stmt, string), fx func(*hir.Expr, string)) {
	if s == nil {
		return
	}
	fs(s, path)
	visitExpr(s.X, path+"/x", fs, fx)
	visitExpr(s.Y, path+"/y", fs, fx)
	visitTree(s.Body, path+"/body", fs, fx)
	visitTree(s.Else, path+"/else", fs, fx)
	for i, v := range s.List {
		visitTree(v, fmt.Sprintf("%s/s%d", path, i), fs, fx)
	}
}
func visitExpr(e *hir.Expr, path string, fs func(*hir.Stmt, string), fx func(*hir.Expr, string)) {
	if e == nil {
		return
	}
	fx(e, path)
	visitExpr(e.X, path+"/0", fs, fx)
	visitExpr(e.Y, path+"/1", fs, fx)
	visitExpr(e.Z, path+"/2", fs, fx)
	for i, a := range e.Args {
		visitExpr(a, fmt.Sprintf("%s/arg%d", path, i), fs, fx)
	}
	if e.Stmt != nil {
		for i, s := range e.Stmt.List {
			visitTree(s, fmt.Sprintf("%s/seq/s%d", path, i), fs, fx)
		}
	}
}
func (r *runner) method(m *hir.Method, depth int) {
	if r.done[m] || m.Body == nil {
		return
	}
	r.done[m] = true
	m.Body = r.stmt(m.Body, m, depth)
}
func (r *runner) stmt(s *hir.Stmt, m *hir.Method, depth int) *hir.Stmt {
	if s == nil || r.err != nil {
		return s
	}
	// A void call can only expand in statement position.
	if s.Kind == hir.ExprStmt && s.X != nil && s.X.Type.Kind == hir.Void {
		r.operands(s.X, m, depth)
		_, out := r.fire(s.X, s, m, depth)
		if out != nil {
			return out
		}
		return s
	}
	s.X = r.expr(s.X, m, depth)
	s.Y = r.expr(s.Y, m, depth)
	s.Body = r.stmt(s.Body, m, depth)
	s.Else = r.stmt(s.Else, m, depth)
	for i, v := range s.List {
		s.List[i] = r.stmt(v, m, depth)
	}
	return s
}
func (r *runner) operands(e *hir.Expr, m *hir.Method, depth int) {
	e.X = r.expr(e.X, m, depth)
	e.Y = r.expr(e.Y, m, depth)
	e.Z = r.expr(e.Z, m, depth)
	for i, a := range e.Args {
		e.Args[i] = r.expr(a, m, depth)
	}
	e.Stmt = r.stmt(e.Stmt, m, depth)
}
func (r *runner) expr(e *hir.Expr, m *hir.Method, depth int) *hir.Expr {
	if e == nil || r.err != nil {
		return e
	}
	r.operands(e, m, depth)
	out, _ := r.fire(e, nil, m, depth)
	if out != nil {
		return out
	}
	return e
}
func (r *runner) fire(e *hir.Expr, s *hir.Stmt, m *hir.Method, depth int) (*hir.Expr, *hir.Stmt) {
	id, original := r.ids[e]
	if !original {
		return nil, nil
	}
	for _, rr := range r.rules.rewrites {
		if rr.bound >= 0 && depth >= rr.bound {
			continue
		}
		env, ok := matches(rr.match, &row{args: Tuple{id, string(e.Kind)}}, map[string]string{})
		if !ok {
			continue
		}
		var choices []Tuple
		c := clause{head: rr.action, body: rr.where, bound: -1}
		join(r.db, r.db, c, -1, 0, env, 0, func(a Tuple, _ int) { choices = append(choices, a) })
		for _, args := range choices {
			var out *hir.Expr
			var stmt *hir.Stmt
			switch rr.action.pred {
			case "replace":
				target := r.nodes[args[1]].expr
				if target == nil || !e.Type.Equal(target.Type) || !inert(e) || !inert(target) {
					continue
				}
				copy := *target
				out = &copy
			case "inline":
				out, stmt = r.inline.expand(e, s, m, depth)
			}
			if out == nil && stmt == nil {
				continue
			}
			r.changed++
			return out, stmt
		}
	}
	return nil, nil
}
func inert(e *hir.Expr) bool {
	return e != nil && (e.Kind == hir.Lit || e.Kind == hir.Local || e.Kind == hir.This)
}
