package rewrite

import (
	"fmt"
	"os"
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
	plans          []*joinPlan
	refresh        map[*hir.Method]bool
	dirty          map[string]bool
}

// Rewrite applies rules deterministically, in operand order and callee-first for
// inline actions. A round visits only its input nodes. Embedded inline rules
// refresh changed method regions and their call dependants between rounds.
// Other rules, or ABAPITI_GRACE_RECOMPUTE=full, use full recomputation.
func Rewrite(p *hir.Program, rules *Rules, limits Limits) (Stats, error) {
	return rewriteObserved(p, rules, limits, nil)
}

func rewriteObserved(p *hir.Program, rules *Rules, limits Limits, observe func(*DB)) (Stats, error) {
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
	selected, demanded, err := rewriteDependencies(rules)
	if err != nil {
		return r.stats, err
	}
	incremental := r.limits.Rounds > 1 && os.Getenv("ABAPITI_GRACE_RECOMPUTE") != "full" && isInlineRules(rules)
	for round := 0; round < r.limits.Rounds; round++ {
		if !incremental || round == 0 {
			r.db = extractDemanded(p, demanded)
			r.refresh = nil
			if incremental {
				r.db.regionFor = r.inlineRegion
			}
		} else {
			affected := r.inlineDependants(r.dirty)
			r.refresh = map[*hir.Method]bool{}
			for id := range affected {
				if m := r.byID[id]; m != nil {
					r.refresh[m] = true
				}
			}
			r.db.invalidateRegions(affected, selected)
			r.db.selections = map[string]regionSelection{}
			for _, c := range selected.clauses {
				pred := c.head.pred
				if inlineRegionalHead(pred) {
					r.db.selections[pred] = regionSelection{column: 0, contains: func(value string) bool { return affected[r.inlineRegionValue(pred, value)] }}
				}
			}
			r.db.evaluateRegion = func(pred string, args Tuple) bool {
				region := r.inlineRegion(pred, args)
				return region == "" || affected[region]
			}
		}
		if err := r.index(); err != nil {
			return r.stats, err
		}
		if err := r.addInlineFacts(); err != nil {
			return r.stats, err
		}
		if err := Evaluate(r.db, selected); err != nil {
			return r.stats, err
		}
		if err := r.validate(); err != nil {
			return r.stats, err
		}
		r.plans = nil
		for _, rr := range r.rules.rewrites {
			c := clause{head: rr.action, body: rr.where, bound: -1}
			r.plans = append(r.plans, planJoin(r.db, c, -1, rr.match.args))
		}
		if observe != nil {
			observe(r.db)
		}
		r.dirty = map[string]bool{}
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
		r.db.evaluateRegion = nil
		r.db.selections = nil
		if !incremental {
			r.db = nil
		}
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
			if t := r.db.tables[a.pred]; t != nil && t.arity != len(a.args) {
				return fmt.Errorf("%s: inconsistent arity", a.pred)
			}
		}
	}
	return nil
}
func (r *runner) index() error {
	kinds := map[string]bool{}
	allNodes := r.rules == nil
	if r.rules != nil {
		for _, rr := range r.rules.rewrites {
			if rr.action.pred == "replace" || rr.match.args[1].variable || rr.match.args[1].wild {
				allNodes = true
			}
			kinds[rr.match.args[1].value] = true
		}
	}
	var addErr error
	addNode := func(path, kind string) {
		if addErr == nil {
			addErr = r.db.Add("node", path, kind)
		}
	}
	if r.refresh == nil {
		r.nodes = map[string]rewriteNode{}
		r.ids = map[*hir.Expr]string{}
		r.methods = map[*hir.Method]string{}
		r.byID = map[string]*hir.Method{}
	} else {
		for id, node := range r.nodes {
			if r.refresh[node.method] {
				delete(r.nodes, id)
				if node.expr != nil {
					delete(r.ids, node.expr)
				}
			}
		}
	}
	for _, c := range r.p.Classes {
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			if r.refresh != nil && !r.refresh[m] {
				continue
			}
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			id := methodID(c.Name, name)
			r.methods[m] = id
			r.byID[id] = m
			visitTree(m.Body, id+"/body", func(s *hir.Stmt, path string) {
				if !allNodes {
					return
				}
				r.nodes[path] = rewriteNode{path, nil, s, m}
				addNode(path, string(s.Kind))
			}, func(e *hir.Expr, path string) {
				if !allNodes && !kinds[string(e.Kind)] {
					return
				}
				r.nodes[path] = rewriteNode{path, e, nil, m}
				r.ids[e] = path
				addNode(path, string(e.Kind))
			})
		}
	}
	return addErr
}

// visitTree uses the fact extractor's site paths, including Seq statement paths.
func visitTree(s *hir.Stmt, path string, fs func(*hir.Stmt, string), fx func(*hir.Expr, string)) {
	if s == nil {
		return
	}
	if path == "" {
		walkSyntax(s, fs, fx)
		return
	}
	fs(s, path)
	if s.X != nil {
		visitExpr(s.X, path+"/x", fs, fx)
	}
	if s.Y != nil {
		visitExpr(s.Y, path+"/y", fs, fx)
	}
	if s.Body != nil {
		visitTree(s.Body, path+"/body", fs, fx)
	}
	if s.Else != nil {
		visitTree(s.Else, path+"/else", fs, fx)
	}
	for i, v := range s.List {
		visitTree(v, fmt.Sprintf("%s/s%d", path, i), fs, fx)
	}
}
func visitExpr(e *hir.Expr, path string, fs func(*hir.Stmt, string), fx func(*hir.Expr, string)) {
	if e == nil {
		return
	}
	fx(e, path)
	if e.X != nil {
		visitExpr(e.X, path+"/0", fs, fx)
	}
	if e.Y != nil {
		visitExpr(e.Y, path+"/1", fs, fx)
	}
	if e.Z != nil {
		visitExpr(e.Z, path+"/2", fs, fx)
	}
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
	for ri, rr := range r.rules.rewrites {
		if rr.bound >= 0 && depth >= rr.bound {
			continue
		}
		env, ok := matches(rr.match, &row{args: Tuple{id, string(e.Kind)}}, map[string]string{})
		if !ok {
			continue
		}
		var choices []Tuple
		r.plans[ri].run(r.db, r.db, env, func(a Tuple, _ int) { choices = append(choices, append(Tuple(nil), a...)) })
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
			r.dirty[r.methods[m]] = true
			r.changed++
			return out, stmt
		}
	}
	return nil, nil
}
func inert(e *hir.Expr) bool {
	return e != nil && (e.Kind == hir.Lit || e.Kind == hir.Local || e.Kind == hir.This)
}

// walkSyntax omits structural path construction for shape-only adapter walks.
func walkSyntax(s *hir.Stmt, fs func(*hir.Stmt, string), fx func(*hir.Expr, string)) {
	if s == nil {
		return
	}
	fs(s, "")
	walkSyntaxExpr(s.X, fs, fx)
	walkSyntaxExpr(s.Y, fs, fx)
	walkSyntax(s.Body, fs, fx)
	walkSyntax(s.Else, fs, fx)
	for _, child := range s.List {
		walkSyntax(child, fs, fx)
	}
}
func walkSyntaxExpr(e *hir.Expr, fs func(*hir.Stmt, string), fx func(*hir.Expr, string)) {
	if e == nil {
		return
	}
	fx(e, "")
	walkSyntaxExpr(e.X, fs, fx)
	walkSyntaxExpr(e.Y, fs, fx)
	walkSyntaxExpr(e.Z, fs, fx)
	for _, child := range e.Args {
		walkSyntaxExpr(child, fs, fx)
	}
	if e.Stmt != nil {
		for _, child := range e.Stmt.List {
			walkSyntax(child, fs, fx)
		}
	}
}
