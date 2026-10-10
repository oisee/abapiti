package rewrite

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/oisee/abapiti/grace"
	"github.com/oisee/abapiti/hir"
)

// AccumulatorSite records the exception proof for an append-only consumer.
type AccumulatorSite struct{ Caller, Site, Callee, Guard string }
type AccumulatorStats struct {
	Clones, Defaults int
	Sites            []AccumulatorSite
}

// PrepareAccumulator is the shared, opt-in pre-emission hook for both targets.
func PrepareAccumulator(p *hir.Program) error {
	if os.Getenv("ABAPITI_ACCUMULATOR") != "1" {
		return nil
	}
	st, err := Accumulator(p)
	if err != nil {
		return err
	}
	if os.Getenv("ABAPITI_ACCUMULATOR_STATS") != "" {
		for _, s := range st.Sites {
			fmt.Fprintf(os.Stderr, "accumulator: %s %s -> %s [%s]\n", s.Caller, s.Site, s.Callee, s.Guard)
		}
		fmt.Fprintf(os.Stderr, "accumulator: %d sites, %d clones, %d defaults\n", len(st.Sites), st.Clones, st.Defaults)
	}
	return nil
}

func accumulatorSignature(m *hir.Method) string {
	var b strings.Builder
	b.WriteString(m.Name)
	for _, a := range m.Params {
		fmt.Fprintf(&b, "|%s:%t", a.Type.String(), a.Variadic)
	}
	b.WriteString("->" + m.Result.String())
	return b.String()
}
func accumulatorCallSignature(e *hir.Expr) string {
	m := &hir.Method{Name: e.Name, Result: e.Type}
	for _, a := range e.Args {
		m.Params = append(m.Params, hir.Param{Type: a.Type})
	}
	return accumulatorSignature(m)
}

// Accumulator adds dispatch-preserving _into variants and rewrites only whole,
// ordered append consumers. Missing proofs leave the original HIR untouched.
func Accumulator(p *hir.Program) (AccumulatorStats, error) {
	var st AccumulatorStats
	db, err := accumulatorAnalysis(p)
	if err != nil {
		return st, err
	}
	hir.AssignSiteIDs(p)
	// Index read bindings and complete dispatch proofs once; Facts returns copies.
	refs := map[string]string{}
	for _, r := range db.Facts("local_ref") {
		refs[r[0]] = r[1]
	}
	noThrow := map[string]bool{}
	for _, r := range db.Facts("calls") {
		safe := db.Has("defined", r[1]) && !db.Has("may_throw", r[1])
		previous, seen := noThrow[r[2]]
		noThrow[r[2]] = safe && (!seen || previous)
	}
	families := map[string][]*hir.Method{}
	owners := map[*hir.Method]*hir.Class{}
	blocked := map[string]bool{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			families[accumulatorSignature(m)] = append(families[accumulatorSignature(m)], m)
			owners[m] = c
			if strings.HasSuffix(m.Name, "_into") {
				blocked[strings.TrimSuffix(m.Name, "_into")] = true
			}
		}
	}
	for _, in := range p.Interfaces {
		for _, m := range in.Methods {
			families[accumulatorSignature(m)] = append(families[accumulatorSignature(m)], m)
			if strings.HasSuffix(m.Name, "_into") {
				blocked[strings.TrimSuffix(m.Name, "_into")] = true
			}
		}
	}
	qualified := map[*hir.Method]accumulatorResultShape{}
	available := map[string]bool{}
	for key, ms := range families {
		for _, m := range ms {
			if m.Static || m.Result.Kind != hir.Array || len(m.Result.Args) != 1 || blocked[m.Name] {
				continue
			}
			variadic := false
			for _, a := range m.Params {
				variadic = variadic || a.Variadic || a.Name == "acc"
			}
			if variadic {
				continue
			}
			c := owners[m]
			if c == nil {
				continue
			}
			id := methodID(c.Name, m.Name)
			if result := accumulatorResult(m, id, db); result.name != "" {
				qualified[m] = result
				available[key] = true
			}
		}
	}
	// Every declaration of this exact name/signature must accept the variant.
	for key := range available {
		for _, m := range families[key] {
			if m.Static || accumulatorReserved(m) {
				delete(available, key)
			}
		}
	}
	used := map[string]bool{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			id := methodID(c.Name, m.Name)
			ids := map[*hir.Expr]string{}
			bindings := map[string]*hir.Stmt{}
			visitTree(m.Body, id+"/body", func(s *hir.Stmt, path string) {
				if s.Kind == hir.VarDecl {
					bindings[path+"/local/"+s.Name] = s
				}
			}, func(e *hir.Expr, path string) { ids[e] = path })
			safe := map[string]bool{}
			for bid, decl := range bindings {
				if db.Has("fresh", id, bid) && accumulatorPrivate(m.Body, decl.Name) && accumulatorOwned(bid, id, db, bindings, ids, refs) {
					safe[bid] = true
				}
			}
			var rewriteStmt func(*hir.Stmt)
			rewriteStmt = func(s *hir.Stmt) {
				if s == nil {
					return
				}
				if s.Kind == hir.Block {
					for i := 0; i < len(s.List); i++ {
						loop := s.List[i]
						if loop == nil {
							continue
						}
						var decl *hir.Stmt
						if loop.Kind == hir.VarDecl && i+1 < len(s.List) {
							decl = loop
							loop = s.List[i+1]
						}
						call, out, lastReads, uses, ok := accumulatorAppend(loop)
						if ok && decl != nil {
							if call.Kind != hir.Local || call.Name != decl.Name {
								ok = false
							} else {
								bid := refs[ids[call]]
								ok = db.Has("use_count", bid, fmt.Sprint(uses))
								for _, read := range lastReads {
									ok = ok && db.Has("not_read_after", bid, ids[read])
								}
								call = decl.X
							}
						}
						if ok && call != nil && call.Kind == hir.VirtualCall && available[accumulatorCallSignature(call)] {
							bid := refs[ids[out]]
							// A private fresh local cannot be reachable through args, fields or statics.
							if safe[bid] && call.Type.Equal(out.Type) {
								noRaise, resolved := noThrow[ids[call]]
								guard := ""
								if resolved && noRaise {
									guard = "may_throw=false"
								} else if resolved && !accumulatorHandlers(m.Body) {
									guard = "fresh local dead on exception"
								}
								if guard != "" {
									key := accumulatorCallSignature(call)
									used[key] = true
									st.Sites = append(st.Sites, AccumulatorSite{id, ids[call], call.Name, guard})
									next := accumulatorCopyExpr(call)
									next.Name += "_into"
									next.Type = hir.T(hir.Void)
									next.Args = append(next.Args, accumulatorCopyExpr(out))
									s.List[i] = &hir.Stmt{Node: loop.Node, Kind: hir.ExprStmt, X: next}
									if decl != nil {
										s.List = append(s.List[:i+1], s.List[i+2:]...)
									}
									continue
								}
							}
						}
						rewriteStmt(s.List[i])
					}
					return
				}
				rewriteStmt(s.Body)
				rewriteStmt(s.Else)
				for _, e := range []*hir.Expr{s.X, s.Y} {
					visitExpr(e, "", func(t *hir.Stmt, _ string) { rewriteStmt(t) }, func(*hir.Expr, string) {})
				}
			}
			rewriteStmt(m.Body)
		}
	}
	// Calls were rewritten before defaults are constructed: their m calls stay m.
	keys := make([]string, 0, len(used))
	for k := range used {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, m := range families[key] {
			params := append([]hir.Param{}, m.Params...)
			params = append(params, hir.Param{Name: "acc", Type: m.Result})
			into := &hir.Method{Node: m.Node, Name: m.Name + "_into", Params: params, Result: hir.T(hir.Void), Virtual: m.Virtual, Internal: m.Internal}
			if c := owners[m]; c != nil {
				if result := qualified[m]; result.name != "" {
					into.Body = accumulatorBody(m.Body, result, m.Result)
					st.Clones++
				} else {
					into.Body = accumulatorDefault(c, m)
					st.Defaults++
				}
				c.Methods = append(c.Methods, into)
			} else {
				for _, in := range p.Interfaces {
					for _, orig := range in.Methods {
						if orig == m {
							in.Methods = append(in.Methods, into)
							break
						}
					}
				}
			}
		}
	}
	if es := hir.Verify(p); len(es) > 0 {
		return st, fmt.Errorf("accumulator produced invalid HIR: %v", es)
	}
	return st, nil
}

func accumulatorConsumer(s *hir.Stmt) (*hir.Expr, *hir.Expr, bool) {
	if s == nil || s.Kind != hir.ForEach || s.X == nil || s.Body == nil {
		return nil, nil, false
	}
	body := s.Body
	if body.Kind == hir.Block {
		if len(body.List) != 1 {
			return nil, nil, false
		}
		body = body.List[0]
	}
	if body.Kind != hir.ExprStmt || body.X == nil {
		return nil, nil, false
	}
	push := body.X
	if push.Kind != hir.RuntimeOp || push.Op != "array.push" || push.X == nil || push.X.Kind != hir.Local || len(push.Args) != 1 {
		return nil, nil, false
	}
	row := push.Args[0]
	if row.Kind != hir.Local || row.Name != s.Name || !row.Type.Equal(s.Type) {
		return nil, nil, false
	}
	return s.X, push.X, true
}

// The pinned combinators use a singleton fast path. Both branches append the
// entire result, so the length test is only a choice of equivalent append code.
// Tests that influence control flow or choose different values do not match.
func accumulatorAppend(s *hir.Stmt) (*hir.Expr, *hir.Expr, []*hir.Expr, int, bool) {
	if call, out, ok := accumulatorConsumer(s); ok {
		return call, out, []*hir.Expr{call}, 1, true
	}
	if s == nil || s.Kind != hir.If || s.X == nil || s.X.Kind != hir.Binary || s.X.Op != "==" {
		return nil, nil, nil, 0, false
	}
	length, one := s.X.X, s.X.Y
	for length != nil && length.Kind == hir.NumericConvert {
		length = length.X
	}
	if length == nil || length.Kind != hir.RuntimeOp || length.Op != "array.length" || length.X == nil || length.X.Kind != hir.Local || !accumulatorInteger(one, 1) {
		return nil, nil, nil, 0, false
	}
	body := s.Body
	if body != nil && body.Kind == hir.Block && len(body.List) == 1 {
		body = body.List[0]
	}
	if body == nil || body.Kind != hir.ExprStmt || body.X == nil {
		return nil, nil, nil, 0, false
	}
	push := body.X
	if push.Kind != hir.RuntimeOp || push.Op != "array.push" || push.X == nil || push.X.Kind != hir.Local || len(push.Args) != 1 {
		return nil, nil, nil, 0, false
	}
	index := push.Args[0]
	if index.Kind != hir.IndexGet || index.X == nil || index.X.Kind != hir.Local || index.X.Name != length.X.Name || !accumulatorInteger(index.Y, 0) {
		return nil, nil, nil, 0, false
	}
	other := s.Else
	if other != nil && other.Kind == hir.Block {
		if len(other.List) == 1 {
			other = other.List[0]
		} else if len(other.List) == 2 && accumulatorSpreadLengths(other)[other.List[1]] {
			other = other.List[0]
		}
	}
	call, out, ok := accumulatorConsumer(other)
	if !ok || call.Kind != hir.Local || call.Name != length.X.Name || out.Name != push.X.Name || !out.Type.Equal(push.X.Type) {
		return nil, nil, nil, 0, false
	}
	return call, out, []*hir.Expr{index.X, call}, 3, true
}
func accumulatorInteger(e *hir.Expr, n int) bool {
	if e != nil && e.Kind == hir.RuntimeOp && e.Op == "number.index" {
		e = e.X
	}
	return e != nil && e.Kind == hir.Lit && (e.Type.Kind == hir.I32 || e.Type.Kind == hir.I64 || e.Type.Kind == hir.Number) && fmt.Sprint(e.Value) == fmt.Sprint(n)
}

// Private identity: no assignments, aliases, closures, field/static stores or
// call operands. Direct returns are allowed only because they leave the caller.
func accumulatorPrivate(body *hir.Stmt, name string) bool {
	good := true
	decls := 0
	visitTree(body, "", func(s *hir.Stmt, _ string) {
		if (s.Kind == hir.VarDecl || s.Kind == hir.ForEach) && s.Name == name {
			decls++
		}
	}, func(e *hir.Expr, _ string) {
		children := []*hir.Expr{e.X, e.Y, e.Z}
		children = append(children, e.Args...)
		for i, c := range children {
			if c != nil && c.Kind == hir.Local && c.Name == name {
				if !(i == 0 && e.Kind == hir.RuntimeOp && (e.Op == "array.push" || e.Op == "array.length")) {
					good = false
				}
			}
		}
	})
	// Inspect statement roots, which have no expression parent.
	visitTree(body, "", func(s *hir.Stmt, _ string) {
		for _, e := range []*hir.Expr{s.X, s.Y} {
			if e != nil && e.Kind == hir.Local && e.Name == name && s.Kind != hir.Return {
				good = false
			}
		}
	}, func(*hir.Expr, string) {})
	return good && decls == 1
}
func accumulatorHandlers(body *hir.Stmt) bool {
	found := false
	visitTree(body, "", func(s *hir.Stmt, _ string) { found = found || s.Kind == hir.Try || s.Kind == hir.Finally }, func(*hir.Expr, string) {})
	return found
}

// The frontend lowers [] to `var tmp = new Array; var result = tmp`.
// Only single-use, last-use initializer links may be collapsed; other aliases
// prevent both the direct clone and the private caller-array proof.
func accumulatorOwned(bid, method string, db *DB, bindings map[string]*hir.Stmt, ids map[*hir.Expr]string, refs map[string]string) bool {
	seen := map[string]bool{}
	for !seen[bid] {
		seen[bid] = true
		decl := bindings[bid]
		if decl == nil || decl.X == nil || !db.Has("fresh", method, bid) {
			return false
		}
		e := decl.X
		if e.Kind == hir.New {
			return e.Type.Kind == hir.Array && len(e.Args) == 0
		}
		if e.Kind != hir.Local {
			return false
		}
		source := refs[ids[e]]
		if !db.Has("must_alias", method, bid, source) || !db.Has("use_count", source, "1") || !db.Has("not_read_after", source, ids[e]) {
			return false
		}
		bid = source
	}
	return false
}

type accumulatorResultShape struct {
	name   string
	prefix int
}

func accumulatorResult(m *hir.Method, id string, db *DB) accumulatorResultShape {
	fail := accumulatorResultShape{}
	if m.Body == nil || m.Body.Kind != hir.Block || len(m.Body.List) < 2 {
		return fail
	}
	decl := m.Body.List[0]
	if decl == nil || decl.Kind != hir.VarDecl || decl.X == nil || decl.X.Kind != hir.New || !decl.Type.Equal(m.Result) || len(decl.X.Args) != 0 {
		return fail
	}
	prefix := 1
	names := map[string]bool{decl.Name: true}
	allowed := map[*hir.Expr]bool{}
	for prefix < len(m.Body.List) {
		next := m.Body.List[prefix]
		if next == nil || next.Kind != hir.VarDecl || next.X == nil || next.X.Kind != hir.Local || next.X.Name != decl.Name || !next.Type.Equal(m.Result) {
			break
		}
		bid := fmt.Sprintf("%s/body/s%d/local/%s", id, prefix-1, decl.Name)
		path := fmt.Sprintf("%s/body/s%d/x", id, prefix)
		if !db.Has("fresh", id, bid) || !db.Has("use_count", bid, "1") || !db.Has("not_read_after", bid, path) {
			return fail
		}
		allowed[next.X] = true
		decl = next
		names[decl.Name] = true
		prefix++
	}
	bid := fmt.Sprintf("%s/body/s%d/local/%s", id, prefix-1, decl.Name)
	if !db.Has("fresh", id, bid) || names["acc"] {
		return fail
	}
	ignored := accumulatorSpreadLengths(m.Body)
	returns := 0
	good := true
	visitTree(m.Body, "", func(s *hir.Stmt, _ string) {
		if (s.Kind == hir.VarDecl || s.Kind == hir.ForEach || s.Kind == hir.Try) && s.Name == "acc" {
			good = false
		}
		if s.Kind == hir.Return {
			if s.X == nil || s.X.Kind != hir.Local || s.X.Name != decl.Name {
				good = false
			} else {
				allowed[s.X] = true
				returns++
			}
		}
		if (s.Kind == hir.VarDecl || s.Kind == hir.ForEach || s.Kind == hir.Try) && names[s.Name] {
			isPrefix := false
			for _, d := range m.Body.List[:prefix] {
				isPrefix = isPrefix || s == d
			}
			if !isPrefix {
				good = false
			}
		}
		if ignored[s] {
			allowed[s.X.X] = true
		}
	}, func(e *hir.Expr, _ string) {
		if e.Kind == hir.RuntimeOp && e.Op == "array.push" && e.X != nil && e.X.Kind == hir.Local && e.X.Name == decl.Name {
			allowed[e.X] = true
		}
	})
	visitTree(m.Body, "", func(*hir.Stmt, string) {}, func(e *hir.Expr, _ string) {
		if e.Kind == hir.Local && (e.Name == "acc" || names[e.Name] && !allowed[e]) {
			good = false
		}
	})
	last := m.Body.List[len(m.Body.List)-1]
	if !good || returns == 0 || last == nil || last.Kind != hir.Return {
		return fail
	}
	return accumulatorResultShape{decl.Name, prefix}
}

// spreadPush's unused length follows its append loop and carries the exact
// same source node. It is a lowering artifact, not a source read of result.
// An explicit length/index read never satisfies this witness.
func accumulatorSpreadLengths(body *hir.Stmt) map[*hir.Stmt]bool {
	ignored := map[*hir.Stmt]bool{}
	visitTree(body, "", func(s *hir.Stmt, _ string) {
		if s.Kind != hir.Block {
			return
		}
		for i := 1; i < len(s.List); i++ {
			prev, read := s.List[i-1], s.List[i]
			_, out, ok := accumulatorConsumer(prev)
			if ok && read != nil && read.Kind == hir.ExprStmt && read.X != nil && read.X.Kind == hir.RuntimeOp && read.X.Op == "array.length" && read.X.X != nil && read.X.X.Kind == hir.Local && read.X.X.Name == out.Name && read.Source != "" && read.Source == prev.Source {
				ignored[read] = true
			}
		}
	}, func(*hir.Expr, string) {})
	return ignored
}
func accumulatorBody(body *hir.Stmt, result accumulatorResultShape, arr hir.Type) *hir.Stmt {
	out := accumulatorCopyStmt(body)
	out.List = out.List[result.prefix:]
	ignored := accumulatorSpreadLengths(out)
	visitTree(out, "", func(s *hir.Stmt, _ string) {
		if s.Kind == hir.Return {
			s.X = nil
		}
		if s.Kind == hir.Block {
			list := s.List[:0]
			for _, c := range s.List {
				if !ignored[c] {
					list = append(list, c)
				}
			}
			s.List = list
		}
	}, func(e *hir.Expr, _ string) {
		if e.Kind == hir.Local && e.Name == result.name {
			e.Name = "acc"
			e.Type = arr
		}
	})
	return out
}
func accumulatorDefault(c *hir.Class, m *hir.Method) *hir.Stmt {
	args := []*hir.Expr{}
	for _, a := range m.Params {
		args = append(args, hir.V(a.Name, a.Type))
	}
	row := "accumulator_row"
	for {
		clash := false
		for _, a := range m.Params {
			clash = clash || a.Name == row
		}
		if !clash {
			break
		}
		row += "_"
	}
	return hir.B(&hir.Stmt{Kind: hir.ForEach, Name: row, Type: m.Result.Args[0], X: &hir.Expr{Kind: hir.VirtualCall, Name: m.Name, Type: m.Result, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}, Args: args}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("acc", m.Result), Args: []*hir.Expr{hir.V(row, m.Result.Args[0])}}})})
}
func accumulatorCopyStmt(s *hir.Stmt) *hir.Stmt {
	if s == nil {
		return nil
	}
	c := *s
	c.X = accumulatorCopyExpr(s.X)
	c.Y = accumulatorCopyExpr(s.Y)
	c.Body = accumulatorCopyStmt(s.Body)
	c.Else = accumulatorCopyStmt(s.Else)
	c.List = nil
	for _, v := range s.List {
		c.List = append(c.List, accumulatorCopyStmt(v))
	}
	return &c
}
func accumulatorCopyExpr(e *hir.Expr) *hir.Expr {
	if e == nil {
		return nil
	}
	c := *e
	c.X = accumulatorCopyExpr(e.X)
	c.Y = accumulatorCopyExpr(e.Y)
	c.Z = accumulatorCopyExpr(e.Z)
	c.Stmt = accumulatorCopyStmt(e.Stmt)
	c.Args = nil
	for _, v := range e.Args {
		c.Args = append(c.Args, accumulatorCopyExpr(v))
	}
	return &c
}

// Only the analysis relations used by this pass are computed. In particular,
// unrelated purity and MayDiverge summaries do not add build cost.
func accumulatorAnalysis(p *hir.Program) (*DB, error) {
	if es := hir.Verify(p); len(es) > 0 {
		return nil, fmt.Errorf("invalid HIR: %v", es)
	}
	source, err := ruleFiles.ReadFile("rules/analysis.grace")
	if err != nil {
		return nil, err
	}
	_, rules, err := Parse(string(source))
	if err != nil {
		return nil, err
	}
	selected, demand := grace.SelectDemand([]*Rules{rules}, []string{"fresh", "local_ref", "use_count", "not_read_after", "calls", "defined", "may_throw", "must_alias"})
	db := extractDemanded(p, demand)
	if err := Evaluate(db, selected); err != nil {
		return nil, err
	}
	return db, nil
}
func accumulatorReserved(m *hir.Method) bool {
	for _, a := range m.Params {
		if a.Variadic || a.Name == "acc" {
			return true
		}
	}
	return false
}
