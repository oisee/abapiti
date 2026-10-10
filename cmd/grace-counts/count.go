package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

type site struct {
	Caller, Path, Source, Stage, Form, Verdict string
	Arg, Loop, Yes                             int
	Hot                                        bool
	Receivers, Targets                         []string
}
type methodUse struct {
	Method, Param, Blocker string
	Arg, Loops             int
	Eligible, Escapes      bool
	Aliases, Flows         int
}
type located struct {
	e           *hir.Expr
	order, loop int
}
type declaration struct {
	input, path string
	order, loop int
	init        *hir.Expr
}

func count(p *hir.Program, db *rewrite.DB, literalSource func(string) bool) ([]site, []methodUse) {
	nodes := map[string]located{}
	decls := map[string]declaration{}
	assignments := map[string][]string{}
	methods := map[string]*hir.Method{}
	for _, c := range p.Classes {
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			id := c.Name + "::" + name
			methods[id] = m
			order := 0
			walk(m.Body, id+"/body", 0, func(s *hir.Stmt, path string, loop int) {
				order++
				if s.Kind == hir.VarDecl {
					decls[path+"/local/"+s.Name] = declaration{path + "/x", path, order, loop, s.X}
				}
			}, func(e *hir.Expr, path string, loop int) { order++; nodes[path] = located{e, order, loop} })
		}
	}
	// Grace bindings resolve lexical scope and retain parameter/alias identity.
	for _, r := range db.Facts("assign") {
		assignments[r[1]] = append(assignments[r[1]], r[2])
	}
	for id, d := range decls {
		if d.init != nil && d.init.Kind == hir.Local {
			var matches []string
			for _, input := range assignments[id] {
				if strings.HasSuffix(input, "/"+d.init.Name) {
					matches = append(matches, input)
				}
			}
			if len(matches) == 1 {
				d.input = matches[0]
			} else {
				d.input = ""
			}
			decls[id] = d
		}
	}
	flow := map[string][]string{}
	for _, r := range db.Facts("flow") {
		flow[r[1]] = append(flow[r[1]], r[2])
	}
	// Resolve the immediate Local receiver of a runtime operation from flow.
	receiverID := func(path string, e *hir.Expr) string {
		if e.X == nil || e.X.Kind != hir.Local {
			return ""
		}
		suffix := "/" + e.X.Name
		for _, id := range flow[path] {
			if strings.HasSuffix(id, suffix) {
				return id
			}
		}
		return ""
	}
	receivers := map[string][]string{}
	dispatch := map[string]string{}
	calls := map[string][]string{}
	for _, r := range db.Facts("receivers") {
		receivers[r[0]] = append(receivers[r[0]], r[1])
	}
	for _, r := range db.Facts("dispatch") {
		dispatch[r[0]+"::"+r[1]] = r[2]
	}
	for _, r := range db.Facts("calls") {
		calls[r[2]] = append(calls[r[2]], r[1])
	}
	cache := map[string]methodUse{}
	var sites []site
	for _, r := range db.Facts("argument") {
		arg, err := strconv.Atoi(r[1])
		if err != nil {
			continue
		}
		n := nodes[r[0]]
		if n.e == nil || !(n.e.Kind == hir.DirectCall || n.e.Kind == hir.VirtualCall || n.e.Kind == hir.SuperCall || n.e.Kind == hir.New && n.e.Type.Kind == hir.ClassRef) {
			continue
		}
		id := r[2]
		form := "literal"
		chain := false
		seen := map[string]bool{}
		valid := true
		// Trace only declaration aliases; assignments invalidate singleton status,
		// except the explicit x = callee(x) loop chain, whose first iteration is S1.
		for {
			if seen[id] {
				valid = false
				break
			}
			seen[id] = true
			if node := nodes[id]; node.e != nil && node.e.Kind == hir.Seq && node.e.Y != nil && node.e.Y.Kind == hir.Local && len(flow[id]) == 1 {
				id = flow[id][0]
				continue
			}
			d := decls[id]
			if d.path == "" {
				break
			}
			if d.order >= n.order {
				valid = false
				break
			}
			if d.init != nil && d.init.Kind != hir.New {
				form = "local"
			}
			for _, a := range assignments[id] {
				if a == d.input {
					continue
				}
				if a == r[0] && n.loop > d.loop {
					chain = true
				} else {
					valid = false
				}
			}
			id = d.input
		}
		if !valid {
			continue
		}
		origin := nodes[id]
		if origin.e == nil || origin.e.Kind != hir.New || origin.e.Type.Kind != hir.Array || len(origin.e.Args) != 0 {
			continue
		}
		if literalSource != nil && !literalSource(origin.e.Source) {
			continue
		}
		// Find the lowered literal's one push. For safety, every intervening use of
		// its container or alias must be declaration aliasing or that push.
		pushes := 0
		unsafe := false
		for path, use := range nodes {
			if use.order <= origin.order || use.order >= n.order {
				continue
			}
			touches := false
			for _, input := range flow[path] {
				if seen[input] || input == id {
					touches = true
				}
			}
			if !touches {
				continue
			}
			if seen[path] && use.e.Kind == hir.Seq {
				continue
			}
			if use.e.Kind == hir.RuntimeOp && use.e.Op == "array.push" && seen[receiverID(path, use.e)] && len(use.e.Args) == 1 && blockOf(path) == blockOf(id) {
				pushes++
			} else {
				unsafe = true
			}
		}
		if pushes != 1 || unsafe {
			continue
		}
		caller := r[0][:strings.Index(r[0], "/body")]
		row := site{Caller: caller, Path: r[0], Source: n.e.Source, Stage: stage(caller), Form: form, Arg: arg, Loop: n.loop, Hot: hot(caller), Verdict: "none"}
		if chain {
			row.Form = "local-first"
		}
		if n.e.Kind == hir.VirtualCall {
			row.Receivers = receivers[r[0]]
			for _, c := range row.Receivers {
				row.Targets = append(row.Targets, dispatch[c+"::"+n.e.Name])
			}
		} else {
			row.Targets = calls[r[0]]
			row.Receivers = append([]string{}, row.Targets...)
		}
		for _, target := range row.Targets {
			key := fmt.Sprintf("%s#%d", target, arg)
			u, ok := cache[key]
			if !ok {
				u = parameterUse(target, methods[target], arg, db)
				cache[key] = u
			}
			if u.Eligible {
				row.Yes++
			}
		}
		if len(row.Targets) > 0 && row.Yes == len(row.Targets) {
			row.Verdict = "qualifies"
		} else if row.Yes > 0 {
			row.Verdict = "partially"
		}
		sites = append(sites, row)
		if chain {
			row.Form = "chained"
			row.Verdict = "not-singleton"
			sites = append(sites, row)
		}
	}
	// Include profiled run methods even when no singleton call dispatches to them.
	for id, m := range methods {
		if hot(id) && m.Name == "run" && len(m.Params) > 0 {
			key := id + "#0"
			if _, ok := cache[key]; !ok {
				cache[key] = parameterUse(id, m, 0, db)
			}
		}
	}
	var uses []methodUse
	for _, u := range cache {
		uses = append(uses, u)
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].Path != sites[j].Path {
			return sites[i].Path < sites[j].Path
		}
		if sites[i].Arg != sites[j].Arg {
			return sites[i].Arg < sites[j].Arg
		}
		return sites[i].Form < sites[j].Form
	})
	sort.Slice(uses, func(i, j int) bool {
		if uses[i].Method != uses[j].Method {
			return uses[i].Method < uses[j].Method
		}
		return uses[i].Arg < uses[j].Arg
	})
	return sites, uses
}

// Literal initialization pushes occur in the same straight-line block as New.
func blockOf(path string) string {
	i := strings.LastIndex(path, "/s")
	if i < 0 {
		return ""
	}
	return path[:i]
}

func parameterUse(id string, m *hir.Method, arg int, db *rewrite.DB) methodUse {
	u := methodUse{Method: id, Arg: arg, Blocker: "unresolved/missing parameter"}
	if m == nil || m.Abstract || m.Body == nil || arg >= len(m.Params) || m.Params[arg].Variadic {
		return u
	}
	u.Param = m.Params[arg].Name
	binding := id + "/param/" + u.Param
	u.Escapes = db.Has("escapes", id, binding)
	for _, r := range db.Facts("alias") {
		if r[0] == id && (r[1] == binding || r[2] == binding) {
			u.Aliases++
		}
	}
	for _, r := range db.Facts("flow") {
		if r[0] == id && (r[1] == binding || r[2] == binding) {
			u.Flows++
		}
	}
	u.Eligible, u.Blocker = true, ""
	// Local names are resolved lexically: declarations and loop binders shadow
	// the parameter. Only a bare Local directly in ForEach.X is permitted.
	var stmt func(*hir.Stmt, string, bool)
	var expr func(*hir.Expr, string, string, bool)
	expr = func(e *hir.Expr, path, context string, active bool) {
		if e == nil {
			return
		}
		if e.Kind == hir.Local && e.Name == u.Param && active {
			if context == "foreach-source" {
				u.Loops++
			} else if u.Blocker == "" {
				u.Blocker = context + " at " + path
				u.Eligible = false
			}
			return
		}
		if e.Kind == hir.Seq && e.Stmt != nil {
			for i, s := range e.Stmt.List {
				stmt(s, fmt.Sprintf("%s/seq/s%d", path, i), active)
				if s != nil && s.Kind == hir.VarDecl && s.Name == u.Param {
					active = false
				}
			}
		}
		kind := string(e.Kind)
		if e.Kind == hir.RuntimeOp {
			kind = e.Op
		}
		expr(e.X, path+"/0", kind+" receiver/operand", active)
		expr(e.Y, path+"/1", kind+" operand", active)
		expr(e.Z, path+"/2", kind+" operand", active)
		for i, a := range e.Args {
			expr(a, fmt.Sprintf("%s/arg%d", path, i), kind+" argument", active)
		}
	}
	stmt = func(s *hir.Stmt, path string, active bool) {
		if s == nil {
			return
		}
		if s.Kind == hir.Block {
			for i, b := range s.List {
				stmt(b, fmt.Sprintf("%s/s%d", path, i), active)
				if b != nil && b.Kind == hir.VarDecl && b.Name == u.Param {
					active = false
				}
			}
			return
		}
		context := string(s.Kind)
		if s.Kind == hir.ForEach {
			context = "foreach-source"
		}
		expr(s.X, path+"/x", context, active)
		expr(s.Y, path+"/y", string(s.Kind)+" value", active)
		shadow := (s.Kind == hir.ForEach || s.Kind == hir.Try) && s.Name == u.Param
		stmt(s.Body, path+"/body", active && !(s.Kind == hir.ForEach && shadow))
		stmt(s.Else, path+"/else", active && !(s.Kind == hir.Try && shadow))
	}
	stmt(m.Body, id+"/body", true)
	return u
}

func stage(id string) string {
	if strings.HasPrefix(id, "src/abap/2_statements/combi.ts.") {
		return "statements/combi." + strings.Split(strings.TrimPrefix(id, "src/abap/2_statements/combi.ts."), "::")[0]
	}
	for _, pair := range [][2]string{{"src/abap/2_statements/", "statements"}, {"src/abap/1_lexer/", "lexer"}, {"src/abap/3_structures/", "structures"}, {"src/abap/5_syntax/", "syntax"}, {"src/rules/", "rules"}} {
		if strings.HasPrefix(id, pair[0]) {
			return pair[1]
		}
	}
	return "other"
}
func hot(id string) bool {
	if !strings.HasPrefix(id, "src/abap/2_statements/combi.ts.") {
		return false
	}
	c := strings.Split(strings.TrimPrefix(id, "src/abap/2_statements/combi.ts."), "::")[0]
	return strings.Contains(" Expression Sequence Vers Token Word Star Alternative Optional Plus Permutation WordSequence ", " "+c+" ")
}
