package abap

import (
	"fmt"

	"github.com/oisee/abapiti/hir"
)

// initChunkNodes bounds one module initializer chunk in HIR nodes (statements
// and expressions); about 32 KB of ABAP. Large modules otherwise produce one
// huge initializer method (frontend.mjs: ~1 MB of Go per method on OSGo).
var initChunkNodes = 2000

// initializerChunks splits a module initializer body into consecutive runs of
// top-level statements and emits each run as its own static method. The
// initializer then sets its flag and calls the chunks in order: evaluation
// order is unchanged, and an exception in one chunk skips every later chunk
// exactly as a JavaScript throw skips the rest of the module. A boundary
// never separates two statements that mention the same local, so chunks
// share no locals. Nil keeps the single-method layout: the body fits one
// chunk, cannot be split, or returns from the initializer. The caller then
// calls the chunks in order and appends the returned chunk methods.
func (e *emitter) initializerChunks(c *hir.Class, m *hir.Method, block *hir.Stmt) []string {
	var runs [][]*hir.Stmt
	size := 0
	for _, group := range localGroups(block.List) {
		n := 0
		for _, s := range group {
			n += stmtNodes(s)
			if containsReturn(s) {
				return nil
			}
		}
		if len(runs) == 0 || size > 0 && size+n > initChunkNodes {
			runs = append(runs, nil)
			size = 0
		}
		runs[len(runs)-1] = append(runs[len(runs)-1], group...)
		size += n
	}
	if len(runs) < 2 {
		return nil
	}
	if e.initChunks == nil {
		e.initChunks = map[string][]string{}
	}
	var out []string
	for i, run := range runs {
		name := e.name(fmt.Sprintf("builtin.initializeChunk.%s.%d", c.Name, i+1))
		e.initChunks[c.Name] = append(e.initChunks[c.Name], name)
		b := &body{e: e, c: c, m: m, implemented: name, locals: map[string]string{}}
		b.stmt(&hir.Stmt{Node: block.Node, Kind: block.Kind, List: run})
		out = append(out, "METHOD "+name+".\n"+b.constantDeclarations()+foldTempInits(b.code.String())+"ENDMETHOD.\n")
	}
	return out
}

// initFailure closes a truncated module initializer (hir.Method.InitFailure):
// the failure is kept and the flag reset, so every later access rethrows it.
func (b *body) initFailure(c *hir.Class, m *hir.Method) {
	if !m.InitFailure {
		return
	}
	if b.e.initFailures == nil {
		b.e.initFailures = map[string]bool{}
	}
	b.e.initFailures[c.Name] = true
	// OSGo catches only into locals or private attributes (0171e92b).
	caught := b.rawTemp("REF TO cx_root")
	b.line("CATCH cx_root INTO " + caught + ".")
	b.line(b.e.name("builtin.initFailure."+c.Name) + " = " + caught + ".")
	b.line(b.e.name("builtin.initialized."+c.Name) + " = abap_false.")
	b.line("RAISE EXCEPTION " + caught + ".")
	b.line("ENDTRY.")
}

// localGroups partitions top-level statements into the smallest contiguous
// groups such that no local name (declared or used) crosses a group boundary.
func localGroups(list []*hir.Stmt) [][]*hir.Stmt {
	last := map[string]int{}
	names := make([]map[string]bool, len(list))
	for i, s := range list {
		names[i] = map[string]bool{}
		stmtLocals(s, names[i])
		for n := range names[i] {
			last[n] = i
		}
	}
	var out [][]*hir.Stmt
	start, end := 0, 0
	for i := range list {
		for n := range names[i] {
			end = max(end, last[n])
		}
		if i == end {
			out = append(out, list[start:i+1])
			start, end = i+1, i+1
		}
	}
	return out
}

// stmtLocals collects every local a statement declares, binds or reads.
func stmtLocals(s *hir.Stmt, into map[string]bool) {
	walkStmt(s, func(s *hir.Stmt) {
		if s.Name != "" && s.Kind != hir.Trap {
			into[s.Name] = true
		}
	}, func(x *hir.Expr) {
		if x.Kind == hir.Local {
			into[x.Name] = true
		}
	})
}

func stmtNodes(s *hir.Stmt) int {
	n := 0
	walkStmt(s, func(*hir.Stmt) { n++ }, func(*hir.Expr) { n++ })
	return n
}

func containsReturn(s *hir.Stmt) bool {
	found := false
	walkStmt(s, func(s *hir.Stmt) { found = found || s.Kind == hir.Return }, func(*hir.Expr) {})
	return found
}

func walkStmt(s *hir.Stmt, stmt func(*hir.Stmt), expr func(*hir.Expr)) {
	if s == nil {
		return
	}
	stmt(s)
	walkExpr(s.X, stmt, expr)
	walkExpr(s.Y, stmt, expr)
	walkStmt(s.Body, stmt, expr)
	walkStmt(s.Else, stmt, expr)
	for _, c := range s.List {
		walkStmt(c, stmt, expr)
	}
}

func walkExpr(x *hir.Expr, stmt func(*hir.Stmt), expr func(*hir.Expr)) {
	if x == nil {
		return
	}
	expr(x)
	walkExpr(x.X, stmt, expr)
	walkExpr(x.Y, stmt, expr)
	walkExpr(x.Z, stmt, expr)
	for _, a := range x.Args {
		walkExpr(a, stmt, expr)
	}
	walkStmt(x.Stmt, stmt, expr)
}
