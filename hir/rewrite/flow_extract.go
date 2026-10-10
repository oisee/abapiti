package rewrite

import (
	"fmt"
	"strconv"

	"github.com/oisee/abapiti/hir"
)

// flowExtractor builds an analysis-only CFG. Nodes are evaluation completion
// points; edges preserve operand order. Exceptional edges overapproximate raises.
type flowExtractor struct {
	x       *extractor
	envs    map[string]scope
	loops   map[string][]string
	uses    map[string]int
	edges   map[string][]string
	keep    map[string]bool
	compact bool
	values  map[string]string
}

func (x *extractor) flowFacts(m *hir.Method, env scope) {
	x.flowFactsMode(m, env, false)
}
func (x *extractor) flowFactsMode(m *hir.Method, env scope, compact bool) {
	f := &flowExtractor{x: x, envs: map[string]scope{}, loops: map[string][]string{}, uses: map[string]int{}, edges: map[string][]string{}, keep: map[string]bool{}, compact: compact, values: map[string]string{}}
	for _, b := range env {
		f.uses[b.id] = 0
		f.define(b.id, x.method+"/entry")
	}
	f.bindStmt(m.Body, x.method+"/body", copyScope(env), nil)
	f.node(x.method+"/entry", env, nil)
	f.edge(x.method+"/entry", f.stmt(m.Body, x.method+"/body", "", "", "", ""))
	f.flushEdges()
	for id, n := range f.uses {
		x.add("use_count", id, strconv.Itoa(n))
	}
}
func (f *flowExtractor) node(path string, env scope, loops []string) {
	if f.compact {
		return
	}
	if !f.compact {
		f.envs[path] = copyScope(env)
	}
	f.loops[path] = append([]string{}, loops...)
	f.x.add("flow_node", f.x.method, path)
	for _, l := range loops {
		f.x.add("in_body", path, l)
	}
}
func (f *flowExtractor) bindExpr(e *hir.Expr, path string, env scope, loops []string) {
	if e == nil {
		return
	}
	f.node(path, env, loops)
	if e.Kind == hir.Seq && e.Stmt != nil {
		env = copyScope(env)
		for i, s := range e.Stmt.List {
			f.bindStmt(s, fmt.Sprintf("%s/seq/s%d", path, i), env, loops)
		}
	}
	if !f.compact {
		f.envs[path] = copyScope(env)
	}
	for i, c := range []*hir.Expr{e.X, e.Y, e.Z} {
		f.bindExpr(c, fmt.Sprintf("%s/%d", path, i), env, loops)
	}
	for i, c := range e.Args {
		f.bindExpr(c, fmt.Sprintf("%s/arg%d", path, i), env, loops)
	}
	if e.Kind == hir.Local || e.Kind == hir.This {
		name := e.Name
		if e.Kind == hir.This {
			name = "this"
		}
		id := env[name].id
		f.values[path] = id
		f.x.add("local_ref", path, id)
	}
	if e.Kind == hir.New {
		f.x.add("allocation", f.x.method, path)
	}
	if e.Kind == hir.RuntimeOp && RuntimeEffects[e.Op].Allocates {
		f.x.add("allocation", f.x.method, path)
	}
	if e.Kind == hir.RuntimeOp {
		target := ""
		effect := RuntimeEffects[e.Op]
		if effect.Writes == "Receiver" {
			target = f.value(e.X, path+"/0")
		}
		if len(effect.Writes) > 4 && effect.Writes[:4] == "Arg(" {
			i, err := strconv.Atoi(effect.Writes[4 : len(effect.Writes)-1])
			if err == nil && i < len(e.Args) {
				target = f.value(e.Args[i], fmt.Sprintf("%s/arg%d", path, i))
			}
		}
		if target != "" {
			f.x.add("site_writes", path, target)
			f.x.add("writes_value", f.x.method, target)
		}
	}
}
func (f *flowExtractor) value(e *hir.Expr, path string) string {
	if e == nil {
		return ""
	}
	if f.compact && (e.Kind == hir.Local || e.Kind == hir.This) {
		return f.values[path]
	}
	if e.Kind == hir.Local {
		return f.envs[path][e.Name].id
	}
	if e.Kind == hir.This {
		return f.envs[path]["this"].id
	}
	return path
}
func (f *flowExtractor) bindStmt(s *hir.Stmt, path string, env scope, loops []string) {
	if s == nil {
		return
	}
	f.node(path, env, loops)
	if s.Kind == hir.Block {
		env = copyScope(env)
		for i, b := range s.List {
			f.bindStmt(b, fmt.Sprintf("%s/s%d", path, i), env, loops)
		}
		return
	}
	f.bindExpr(s.X, path+"/x", env, loops)
	f.bindExpr(s.Y, path+"/y", env, loops)
	if s.Kind == hir.VarDecl {
		id := path + "/local/" + s.Name
		env[s.Name] = binding{id: id}
		f.uses[id] = 0
		f.define(id, path)
	}
	if s.Kind == hir.Assign && s.X != nil {
		if s.X.Kind == hir.Local {
			f.define(env[s.X.Name].id, path)
		} else if s.X.Kind == hir.FieldGet || s.X.Kind == hir.IndexGet {
			target := f.value(s.X.X, path+"/x/0")
			f.x.add("writes_value", f.x.method, target)
			f.x.add("site_writes", path, target)
		}
	}
	body, other := env, env
	if !f.compact || s.Body != nil || s.Kind == hir.ForEach {
		body = copyScope(env)
	}
	if !f.compact || s.Else != nil || s.Kind == hir.Try {
		other = copyScope(env)
	}
	if s.Kind == hir.ForEach || s.Kind == hir.While {
		f.x.add("diverge_seed", f.x.method)
		loops = append(append([]string{}, loops...), path)
	}
	if s.Kind == hir.ForEach {
		id := path + "/local/" + s.Name
		body[s.Name] = binding{id: id}
		f.x.add("loop", path, f.value(s.X, path+"/x"), id)
		f.uses[id] = 0
		f.define(id, path+"/row")
		f.node(path+"/row", body, loops)
	}
	if s.Kind == hir.Try {
		other[s.Name] = binding{id: path + "/catch/" + s.Name}
		f.uses[other[s.Name].id] = 0
		f.define(other[s.Name].id, path+"/catch")
		f.node(path+"/catch", other, loops)
	}
	f.bindStmt(s.Body, path+"/body", body, loops)
	// Only loop bodies belong to their loop, not its following/alternative path.
	elseLoops := f.loops[path]
	f.bindStmt(s.Else, path+"/else", other, elseLoops)
}
func (f *flowExtractor) edge(a, b string) {
	if a != "" && b != "" {
		f.edges[a] = append(f.edges[a], b)
	}
}
func (f *flowExtractor) read(id, path string) {
	if id == "" {
		return
	}
	f.keep[path] = true
	f.x.add("use", id, path)
	f.x.add("observed", id, path)
	f.uses[id]++
}
func (f *flowExtractor) expr(e *hir.Expr, path, next, ex, brk, cont string, read bool) string {
	if e == nil {
		return next
	}
	f.edge(path, next)
	// Locals/literals cannot raise. Other expression completions may reach the
	// enclosing handler; overapproximation also covers traps and nested calls.
	if e.Kind != hir.Local && e.Kind != hir.This && e.Kind != hir.Lit {
		f.edge(path, ex)
	}
	if read && (e.Kind == hir.Local || e.Kind == hir.This) {
		f.read(f.value(e, path), path)
	}
	if e.Kind == hir.Conditional {
		y := f.expr(e.Y, path+"/1", path, ex, brk, cont, true)
		z := f.expr(e.Z, path+"/2", path, ex, brk, cont, true)
		fork := path + "/branch"
		f.edge(fork, y)
		f.edge(fork, z)
		return f.expr(e.X, path+"/0", fork, ex, brk, cont, true)
	}
	if e.Kind == hir.Binary && (e.Op == "&&" || e.Op == "||") {
		y := f.expr(e.Y, path+"/1", path, ex, brk, cont, true)
		fork := path + "/branch"
		f.edge(fork, y)
		f.edge(fork, path)
		return f.expr(e.X, path+"/0", fork, ex, brk, cont, true)
	}
	entry := path
	for i := len(e.Args) - 1; i >= 0; i-- {
		entry = f.expr(e.Args[i], fmt.Sprintf("%s/arg%d", path, i), entry, ex, brk, cont, true)
	}
	entry = f.expr(e.Z, path+"/2", entry, ex, brk, cont, true)
	entry = f.expr(e.Y, path+"/1", entry, ex, brk, cont, true)
	entry = f.expr(e.X, path+"/0", entry, ex, brk, cont, true)
	if e.Kind == hir.Seq && e.Stmt != nil {
		for i := len(e.Stmt.List) - 1; i >= 0; i-- {
			entry = f.stmt(e.Stmt.List[i], fmt.Sprintf("%s/seq/s%d", path, i), entry, brk, cont, ex)
		}
	}
	return entry
}
func (f *flowExtractor) stmt(s *hir.Stmt, path, next, brk, cont, ex string) string {
	if s == nil {
		return next
	}
	if s.Kind == hir.Block {
		entry := next
		for i := len(s.List) - 1; i >= 0; i-- {
			entry = f.stmt(s.List[i], fmt.Sprintf("%s/s%d", path, i), entry, brk, cont, ex)
		}
		f.edge(path, entry)
		return path
	}
	switch s.Kind {
	case hir.If:
		f.edge(path, f.stmt(s.Body, path+"/body", next, brk, cont, ex))
		f.edge(path, f.stmt(s.Else, path+"/else", next, brk, cont, ex))
	case hir.While, hir.ForEach:
		header := path + "/header"
		f.edge(path, next)
		body := f.stmt(s.Body, path+"/body", header, next, header, ex)
		if s.Kind == hir.ForEach {
			f.edge(path+"/row", body)
			body = path + "/row"
		}
		f.edge(path, body)
		if s.Kind == hir.ForEach {
			f.edge(header, path)
			return f.expr(s.X, path+"/x", header, ex, brk, cont, true)
		}
		entry := f.expr(s.X, path+"/x", path, ex, brk, cont, true)
		f.edge(header, entry)
		return header
	case hir.Try:
		caught := f.stmt(s.Else, path+"/else", next, brk, cont, ex)
		f.edge(path+"/catch", caught)
		f.edge(path, f.stmt(s.Body, path+"/body", next, brk, cont, path+"/catch"))
		return path
	case hir.Finally:
		// HIR forbids return/break/continue in Body. The shared finalizer joins
		// normal and exceptional continuations, conservatively retaining both.
		exit := path + "/finally_exit"
		f.edge(exit, next)
		f.edge(exit, ex)
		fin := f.stmt(s.Else, path+"/else", exit, brk, cont, ex)
		f.edge(path, f.stmt(s.Body, path+"/body", fin, brk, cont, fin))
		return path
	case hir.Return: // no continuation: unreachable later reads do not block
	case hir.Throw, hir.Trap:
		f.edge(path, ex)
	case hir.Break:
		f.edge(path, brk)
	case hir.Continue:
		f.edge(path, cont)
	default:
		f.edge(path, next)
	}
	entry := f.expr(s.Y, path+"/y", path, ex, brk, cont, true)
	read := !(s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local)
	entry = f.expr(s.X, path+"/x", entry, ex, brk, cont, read)
	return entry
}

// Definitions and actual reads are the only liveness transfer/observation
// points. Contract all intervening evaluation/branch nodes into path edges.
// A separate traversal for each source preserves branch correlations and cycles;
// stopping at kept nodes preserves kills and left-to-right operand order.
func (f *flowExtractor) define(id, site string) {
	f.keep[site] = true
	f.x.add("def", id, site)
}
func (f *flowExtractor) flushEdges() {
	for from := range f.keep {
		seen := map[string]bool{}
		pending := append([]string{}, f.edges[from]...)
		for len(pending) > 0 {
			to := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[to] {
				continue
			}
			seen[to] = true
			if f.keep[to] {
				f.x.add("next", from, to)
				continue
			}
			pending = append(pending, f.edges[to]...)
		}
	}
}
