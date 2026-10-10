package rewrite

import (
	"fmt"
	"strconv"

	"github.com/oisee/abapiti/hir"
)

// flowExtractor builds an analysis-only CFG. Nodes are evaluation completion
// points; edges preserve operand order. Exceptional edges overapproximate raises.
type flowExtractor struct {
	x     *extractor
	envs  map[string]scope
	loops map[string][]string
	uses  map[string]int
}

func (x *extractor) flowFacts(m *hir.Method, env scope) {
	f := &flowExtractor{x: x, envs: map[string]scope{}, loops: map[string][]string{}, uses: map[string]int{}}
	for _, b := range env {
		f.uses[b.id] = 0
	}
	f.bindStmt(m.Body, x.method+"/body", copyScope(env), nil)
	f.stmt(m.Body, x.method+"/body", "", "", "", "")
	for id, n := range f.uses {
		x.add("use_count", id, strconv.Itoa(n))
	}
}
func (f *flowExtractor) node(path string, env scope, loops []string) {
	f.envs[path] = copyScope(env)
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
	f.envs[path] = copyScope(env)
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
		f.x.add("def", id, path)
	}
	if s.Kind == hir.Assign && s.X != nil {
		if s.X.Kind == hir.Local {
			f.x.add("def", env[s.X.Name].id, path)
		} else if s.X.Kind == hir.FieldGet || s.X.Kind == hir.IndexGet {
			target := f.value(s.X.X, path+"/x/0")
			f.x.add("writes_value", f.x.method, target)
			f.x.add("site_writes", path, target)
		}
	}
	body := copyScope(env)
	other := copyScope(env)
	if s.Kind == hir.ForEach || s.Kind == hir.While {
		f.x.add("diverge_seed", f.x.method)
		loops = append(append([]string{}, loops...), path)
	}
	if s.Kind == hir.ForEach {
		id := path + "/local/" + s.Name
		body[s.Name] = binding{id: id}
		f.x.add("loop", path, f.value(s.X, path+"/x"), id)
		f.uses[id] = 0
		f.x.add("def", id, path+"/row")
		f.node(path+"/row", body, loops)
	}
	if s.Kind == hir.Try {
		other[s.Name] = binding{id: path + "/catch/" + s.Name}
		f.x.add("def", other[s.Name].id, path+"/catch")
		f.node(path+"/catch", other, loops)
	}
	f.bindStmt(s.Body, path+"/body", body, loops)
	// Only loop bodies belong to their loop, not its following/alternative path.
	elseLoops := f.loops[path]
	f.bindStmt(s.Else, path+"/else", other, elseLoops)
}
func (f *flowExtractor) edge(a, b string) {
	if a != "" && b != "" {
		f.x.add("next", a, b)
	}
}
func (f *flowExtractor) read(id, path string) {
	if id == "" {
		return
	}
	f.x.add("use", id, path)
	f.x.add("observed", id, path)
	f.uses[id]++
}
func (f *flowExtractor) expr(e *hir.Expr, path, next, ex string, read bool) string {
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
		y := f.expr(e.Y, path+"/1", path, ex, true)
		z := f.expr(e.Z, path+"/2", path, ex, true)
		fork := path + "/branch"
		f.edge(fork, y)
		f.edge(fork, z)
		return f.expr(e.X, path+"/0", fork, ex, true)
	}
	if e.Kind == hir.Binary && (e.Op == "&&" || e.Op == "||") {
		y := f.expr(e.Y, path+"/1", path, ex, true)
		fork := path + "/branch"
		f.edge(fork, y)
		f.edge(fork, path)
		return f.expr(e.X, path+"/0", fork, ex, true)
	}
	entry := path
	for i := len(e.Args) - 1; i >= 0; i-- {
		entry = f.expr(e.Args[i], fmt.Sprintf("%s/arg%d", path, i), entry, ex, true)
	}
	entry = f.expr(e.Z, path+"/2", entry, ex, true)
	entry = f.expr(e.Y, path+"/1", entry, ex, true)
	entry = f.expr(e.X, path+"/0", entry, ex, true)
	if e.Kind == hir.Seq && e.Stmt != nil {
		for i := len(e.Stmt.List) - 1; i >= 0; i-- {
			entry = f.stmt(e.Stmt.List[i], fmt.Sprintf("%s/seq/s%d", path, i), entry, "", "", ex)
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
			return f.expr(s.X, path+"/x", header, ex, true)
		}
		entry := f.expr(s.X, path+"/x", path, ex, true)
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
	entry := f.expr(s.Y, path+"/y", path, ex, true)
	read := !(s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local)
	entry = f.expr(s.X, path+"/x", entry, ex, read)
	return entry
}
