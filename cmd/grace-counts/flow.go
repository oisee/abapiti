package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

type flowSite struct {
	Rule, Caller, Path, Source, Stage, Row, RowType, Producer, Consumer, Verdict, Blocker string
	Loop                                                                                  int
}
type flowNode struct {
	e                    *hir.Expr
	s                    *hir.Stmt
	caller, parent, role string
	loop                 int
}
type identities struct{ parent map[string]string }

func (u *identities) find(s string) string {
	p, ok := u.parent[s]
	if !ok {
		u.parent[s] = s
		return s
	}
	if p != s {
		u.parent[s] = u.find(p)
	}
	return u.parent[s]
}
func (u *identities) join(a, b string) {
	a = u.find(a)
	b = u.find(b)
	if a != b {
		if a > b {
			a, b = b, a
		}
		u.parent[b] = a
	}
}

// flowCounts reports conservative HIR opportunities. It never edits a node.
func flowCounts(p *hir.Program, db *rewrite.DB, dir string) []flowSite {
	nodes := map[string]flowNode{}
	var scanStmt func(*hir.Stmt, string, string, int, string, string)
	var scanExpr func(*hir.Expr, string, string, int, string, string)
	scanExpr = func(e *hir.Expr, path, caller string, depth int, parent, role string) {
		if e == nil {
			return
		}
		nodes[path] = flowNode{e: e, caller: caller, loop: depth, parent: parent, role: role}
		if e.Kind == hir.Seq && e.Stmt != nil {
			for i, s := range e.Stmt.List {
				scanStmt(s, fmt.Sprintf("%s/seq/s%d", path, i), caller, depth, path, "seq")
			}
		}
		for i, c := range []*hir.Expr{e.X, e.Y, e.Z} {
			scanExpr(c, fmt.Sprintf("%s/%d", path, i), caller, depth, path, strconv.Itoa(i))
		}
		for i, c := range e.Args {
			scanExpr(c, fmt.Sprintf("%s/arg%d", path, i), caller, depth, path, "arg"+strconv.Itoa(i))
		}
	}
	scanStmt = func(s *hir.Stmt, path, caller string, depth int, parent, role string) {
		if s == nil {
			return
		}
		nodes[path] = flowNode{s: s, caller: caller, loop: depth, parent: parent, role: role}
		scanExpr(s.X, path+"/x", caller, depth, path, "x")
		scanExpr(s.Y, path+"/y", caller, depth, path, "y")
		for i, b := range s.List {
			scanStmt(b, fmt.Sprintf("%s/s%d", path, i), caller, depth, path, "list")
		}
		d := depth
		if s.Kind == hir.ForEach || s.Kind == hir.While {
			d++
		}
		scanStmt(s.Body, path+"/body", caller, d, path, "body")
		scanStmt(s.Else, path+"/else", caller, depth, path, "else")
	}
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
			scanStmt(m.Body, id+"/body", id, 0, "", "")
		}
	}
	refs := map[string]string{}
	for _, r := range db.Facts("local_ref") {
		refs[r[0]] = r[1]
	}
	value := func(path string) string {
		if id := refs[path]; id != "" {
			return id
		}
		return path
	}
	u := &identities{parent: map[string]string{}}
	for _, r := range db.Facts("alias") {
		u.join(r[1], r[2])
	}
	escaped := map[string]bool{}
	for _, r := range db.Facts("escapes") {
		escaped[u.find(r[1])] = true
	}
	fresh := map[string]bool{}
	for _, r := range db.Facts("fresh") {
		fresh[u.find(r[1])] = true
	}
	bindings := map[string][]string{}
	for _, r := range db.Facts("local") {
		bindings[u.find(r[1])] = append(bindings[u.find(r[1])], r[1])
	}
	defs := map[string][]string{}
	for _, r := range db.Facts("def") {
		defs[r[0]] = append(defs[r[0]], r[1])
	}
	uses := map[string][]string{}
	for _, r := range db.Facts("use") {
		uses[u.find(r[0])] = append(uses[u.find(r[0])], r[1])
	}
	writes := map[string][]string{}
	for _, r := range db.Facts("site_writes") {
		writes[r[0]] = append(writes[r[0]], r[1])
	}
	calls := map[string][]string{}
	for _, r := range db.Facts("calls") {
		calls[r[2]] = append(calls[r[2]], r[1])
	}
	loops := map[string][]string{}
	for _, r := range db.Facts("in_body") {
		loops[r[0]] = append(loops[r[0]], r[1])
	}
	mk := func(rule, path string) flowSite {
		n := nodes[path]
		source := ""
		if n.e != nil {
			source = n.e.Source
		}
		if n.s != nil {
			source = n.s.Source
		}
		return flowSite{Rule: rule, Caller: n.caller, Path: path, Source: strings.TrimPrefix(source, dir+string(filepath.Separator)), Stage: stage(n.caller), Loop: n.loop, Verdict: "qualifies"}
	}
	reject := func(s *flowSite, why string) {
		if s.Blocker == "" {
			s.Verdict = "none"
			s.Blocker = why
		}
	}
	// Index blockers by enclosing loop once, rather than scanning the whole
	// closure for each loop. Sorted traversal gives stable first evidence.
	owned := map[string]bool{}
	for g := range fresh {
		owned[g] = !escaped[g]
	}
	for _, r := range db.Facts("param") {
		owned[u.find(r[2])] = false
	}
	for id, ds := range defs {
		if len(ds) > 1 {
			owned[u.find(id)] = false
		}
	}
	bodyWrites := map[string][]string{}
	bodyGaps := map[string][]string{}
	for _, r := range db.Facts("ensure_init") {
		for _, l := range loops[r[1]] {
			if db.Has("writes", r[2]+"::class_constructor", "any") {
				bodyGaps[l] = append(bodyGaps[l], "ensure_init may write any: "+r[2])
			}
		}
	}

	paths := []string{}
	for path := range nodes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		for _, l := range loops[path] {
			for _, v := range writes[path] {
				bodyWrites[l] = append(bodyWrites[l], path+"\x00"+v)
			}
			for _, target := range calls[path] {
				if db.Has("writes", target, "any") {
					bodyGaps[l] = append(bodyGaps[l], "callee may write any: "+target)
				}
			}
			e := nodes[path].e
			if e == nil {
				continue
			}
			if (e.Kind == hir.VirtualCall || e.Kind == hir.DirectCall || e.Kind == hir.SuperCall) && len(calls[path]) == 0 {
				bodyGaps[l] = append(bodyGaps[l], "unresolved call at "+path)
			}
			if e.Kind == hir.RuntimeOp {
				if _, ok := rewrite.RuntimeEffects[e.Op]; !ok || e.Op == "classvalue.new" || e.Op == "dynamic.materialize" {
					bodyGaps[l] = append(bodyGaps[l], "unknown runtime effects at "+path)
				}
			}
		}
	}
	var out []flowSite
	for _, r := range db.Facts("loop") {
		s := mk("R1", r[0])
		n := nodes[r[0]]
		s.Consumer = "foreach"
		s.Row = rowKind(n.s.Type)
		s.RowType = n.s.Type.String()
		group := u.find(r[1])
		row := u.find(r[2])
		if escaped[row] {
			reject(&s, "row escapes")
		}
		for _, path := range defs[r[2]] {
			if path != r[0]+"/row" {
				reject(&s, "row reassigned at "+path)
			}
		}
		for _, w := range bodyWrites[r[0]] {
			parts := strings.SplitN(w, "\x00", 2)
			path, v := parts[0], parts[1]
			g := u.find(v)
			if g == group {
				reject(&s, "iterated array/alias written at "+path)
			}
			if g == row {
				reject(&s, "row/alias written at "+path)
			}
			if g != group && !owned[group] && !owned[g] {
				reject(&s, "possible external alias write at "+path)
			}
		}
		for _, why := range bodyGaps[r[0]] {
			reject(&s, why)
		}
		if s.Row == "unknown" {
			reject(&s, "row representation unknown")
		}
		out = append(out, s)
	}
	// Immediate operands and local bindings are both included. Forwarding through
	// declaration aliases and Seq yields is transparent; construction pushes are
	// allowed, while all other reads must be the single fusable consumer.
	producerGroups := map[string]int{}
	for path, n := range nodes {
		if n.e != nil && n.e.Type.Kind == hir.Array && (n.e.Kind == hir.New || n.e.Kind == hir.RuntimeOp && rewrite.RuntimeEffects[n.e.Op].Allocates && rewrite.RuntimeEffects[n.e.Op].Aliases == "None") {
			producerGroups[u.find(path)]++
		}
	}
	consumer := func(path string) (string, string) {
		n := nodes[path]
		parent := nodes[n.parent]
		if parent.s != nil && parent.s.Kind == hir.ForEach && n.role == "x" {
			return n.parent, "foreach"
		}
		if parent.e != nil && parent.e.Kind == hir.RuntimeOp {
			if parent.e.Op == "array.join" && n.role == "0" {
				return n.parent, "join"
			}
			if parent.e.Op == "array.concat" && (n.role == "0" || strings.HasPrefix(n.role, "arg")) {
				return n.parent, "concat-source"
			}
			if (parent.e.Op == "array.pushAll" || parent.e.Op == "array.push_all") && strings.HasPrefix(n.role, "arg") {
				return n.parent, "push-all-source"
			}
		}
		return "", ""
	}
	transparent := func(path string) bool {
		n := nodes[path]
		par := nodes[n.parent]
		return par.s != nil && par.s.Kind == hir.VarDecl && n.role == "x" || par.e != nil && par.e.Kind == hir.Seq && n.role == "1"
	}
	for path, n := range nodes {
		if n.e == nil {
			continue
		}
		e := n.e
		producer := db.Has("fresh", n.caller, path)
		if producer && e.Type.Kind == hir.Array {
			s := mk("R2", path)
			if len(e.Type.Args) == 1 {
				s.RowType = e.Type.Args[0].String()
			}
			s.Producer = string(e.Kind)
			if e.Kind == hir.RuntimeOp {
				s.Producer = e.Op
			}
			g := u.find(path)
			if escaped[g] {
				reject(&s, "array or alias escapes")
			}
			if producerGroups[g] != 1 {
				reject(&s, "multiple possible producers")
			}
			consumers := map[string]string{}
			observedConsumers := map[string]bool{}
			consider := func(use string) {
				if transparent(use) {
					return
				}
				un := nodes[use]
				par := nodes[un.parent]
				if par.e != nil && par.e.Kind == hir.RuntimeOp && par.e.Op == "array.push" && un.role == "0" {
					return
				}
				if c, k := consumer(use); c != "" {
					consumers[c] = k
					if nodes[c].loop > n.loop {
						reject(&s, "consumer repeats in nested loop")
					}
					if id := refs[use]; id != "" && !db.Has("not_read_after", id, use) {
						reject(&s, "array read after consumer")
					}
				} else {
					kind := "unknown"
					if par.e != nil {
						kind = string(par.e.Kind)
						if par.e.Kind == hir.RuntimeOp {
							kind = par.e.Op
						}
					}
					if par.s != nil {
						kind = string(par.s.Kind)
					}
					observedConsumers[kind] = true
					reject(&s, "nonfusable use at "+use)
				}
			}
			if !transparent(path) {
				consider(path)
			}
			for _, use := range uses[g] {
				consider(use)
			}
			for _, id := range bindings[g] {
				if len(defs[id]) > 1 {
					reject(&s, "array binding reassigned")
				}
			}
			if len(consumers) != 1 {
				kinds := []string{}
				for k := range observedConsumers {
					kinds = append(kinds, k)
				}
				for _, k := range consumers {
					kinds = append(kinds, k)
				}
				sort.Strings(kinds)
				s.Consumer = strings.Join(kinds, ";")
				reject(&s, fmt.Sprintf("%d consumers, want one", len(consumers)))
			} else {
				for c, k := range consumers {
					s.Consumer = k
					s.Consumer += " at " + c
				}
			}
			out = append(out, s)
		}
		if e.Kind == hir.RuntimeOp {
			sources := []string{}
			kind := ""
			switch e.Op {
			case "array.slice0":
				sources = []string{path + "/0"}
				kind = "whole-container-copy"
			case "set.copy":
				sources = []string{path + "/arg0"}
				kind = "whole-container-copy"
			case "array.push":
				for i := range e.Args {
					sources = append(sources, fmt.Sprintf("%s/arg%d", path, i))
				}
				kind = "append-value"
			case "array.concat":
				sources = append(sources, path+"/0")
				for i := range e.Args {
					sources = append(sources, fmt.Sprintf("%s/arg%d", path, i))
				}
				kind = "concat-source"
			}
			for i, src := range sources {
				s := mk("R3", path)
				s.Producer = e.Op
				s.Consumer = kind
				s.Path = fmt.Sprintf("%s/copy%d", path, i)
				id := value(src)
				g := u.find(id)
				sourceNode := nodes[src]
				if sourceNode.e != nil {
					s.RowType = sourceNode.e.Type.String()
				}
				primitive := sourceNode.e != nil && rowKind(sourceNode.e.Type) == "value"
				if primitive {
					// Primitive HIR values have no object-reference escape. Require a
					// single read of a declared local, excluding externally owned params
					// and previous value copies as a conservative ownership proof.
					s.Row = "value"
					if !strings.Contains(id, "/local/") || !db.Has("use_count", id, "1") || len(defs[id]) != 1 {
						reject(&s, "value source ownership/sharing unproved")
					}
				} else {
					if !owned[g] {
						reject(&s, "source ownership unproved")
					}
					if escaped[g] {
						reject(&s, "source or alias escapes")
					}
					if len(bindings[g]) != 1 {
						reject(&s, "source has other reference bindings")
					}
					if kind == "append-value" {
						reject(&s, "reference append has no kernel value-copy gain")
					}
				}
				if refs[src] == "" || !db.Has("not_read_after", id, src) {
					reject(&s, "source may be read after copy")
				}
				out = append(out, s)
			}
		}

	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func rowKind(t hir.Type) string {
	switch t.Kind {
	case hir.String, hir.Bool, hir.Number, hir.I32, hir.I64:
		return "value"
	case hir.ClassRef, hir.InterfaceRef:
		return "object-reference"
	case hir.Optional:
		if len(t.Args) == 1 {
			return rowKind(t.Args[0])
		}
	}
	return "unknown"
}
func writeFlowOutputs(out string, sites []flowSite, db *rewrite.DB) (string, error) {
	rows := [][]string{{"rule", "caller", "site", "source", "stage", "loop_depth", "row_category", "row_type", "producer", "consumer", "verdict", "blocker"}}
	for _, s := range sites {
		rows = append(rows, []string{s.Rule, s.Caller, s.Path, s.Source, s.Stage, strconv.Itoa(s.Loop), s.Row, s.RowType, s.Producer, s.Consumer, s.Verdict, s.Blocker})
	}
	if err := writeCSV(filepath.Join(out, "flow-sites.csv"), rows); err != nil {
		return "", err
	}
	rows = [][]string{{"method", "writes", "allocates", "may_raise", "may_diverge"}}
	for _, r := range db.Facts("writes") {
		rows = append(rows, []string{r[0], r[1], strconv.FormatBool(db.Has("allocates", r[0])), strconv.FormatBool(db.Has("may_raise", r[0])), strconv.FormatBool(db.Has("may_diverge", r[0]))})
	}
	if err := writeCSV(filepath.Join(out, "flow-methods.csv"), rows); err != nil {
		return "", err
	}
	report := flowTables(sites)
	var facts strings.Builder
	fmt.Fprintln(&facts, "\nFlow fact counts (read-only analysis)")
	for _, pred := range []string{"loop", "in_body", "def", "use", "use_count", "not_read_after", "writes_value", "writes_param", "site_writes", "must_alias", "allocates", "may_raise", "may_diverge"} {
		fmt.Fprintf(&facts, "%s | %d\n", pred, db.Count(pred))
	}
	summary := map[string]int{}
	for _, r := range db.Facts("writes") {
		summary[r[1]]++
	}
	fmt.Fprintf(&facts, "Method writes: none=%d own_fields=%d any=%d total=%d\n", summary["none"], summary["own_fields"], summary["any"], db.Count("writes"))
	report += facts.String()
	return report, nil
}
func flowTables(sites []flowSite) string {
	var b strings.Builder
	for _, rule := range []string{"R1", "R2", "R3"} {
		fmt.Fprintf(&b, "\n%s candidates (static HIR sites)\nCaller stage/class | in-loop | outside | all | some | none | value yes | ref yes\n", rule)
		counts := map[string][7]int{}
		for _, k := range []string{"lexer", "statements", "structures", "syntax", "rules", "other"} {
			counts[k] = [7]int{}
		}
		for _, class := range strings.Fields("Expression Sequence Vers Token Word Star Alternative Optional Plus Permutation WordSequence") {
			counts["statements/combi."+class] = [7]int{}
		}
		for _, s := range sites {
			if s.Rule != rule {
				continue
			}
			n := counts[s.Stage]
			if s.Loop > 0 {
				n[0]++
			} else {
				n[1]++
			}
			if s.Verdict == "qualifies" {
				n[2]++
				if s.Row == "value" {
					n[5]++
				}
				if s.Row == "object-reference" {
					n[6]++
				}
			} else {
				n[4]++
			}
			counts[s.Stage] = n
		}
		keys := []string{}
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n := counts[k]
			fmt.Fprintf(&b, "%s | %d | %d | %d | %d | %d | %d | %d\n", k, n[0], n[1], n[2], n[3], n[4], n[5], n[6])
		}
	}
	fmt.Fprintln(&b, "\nTop 20 lexer + statements sites per rule (qualifying value gains first, then references, then blocked; in-loop first within each group)")
	for _, rule := range []string{"R1", "R2", "R3"} {
		selected := []flowSite{}
		for _, s := range sites {
			if s.Rule == rule && (s.Stage == "lexer" || strings.HasPrefix(s.Stage, "statements")) {
				selected = append(selected, s)
			}
		}
		sort.Slice(selected, func(i, j int) bool {
			priority := func(s flowSite) int {
				if s.Verdict != "qualifies" {
					return 0
				}
				if s.Rule == "R1" && s.Row == "object-reference" {
					return 1
				}
				return 2
			}
			if priority(selected[i]) != priority(selected[j]) {
				return priority(selected[i]) > priority(selected[j])
			}
			if (selected[i].Loop > 0) != (selected[j].Loop > 0) {
				return selected[i].Loop > 0
			}
			return selected[i].Path < selected[j].Path
		})
		if len(selected) > 20 {
			selected = selected[:20]
		}
		for _, s := range selected {
			fmt.Fprintf(&b, "%s | %s | depth=%d | %s | %s -> %s | %s | %s | %s\n", s.Rule, s.Path, s.Loop, s.Row, s.Producer, s.Consumer, s.Verdict, s.Blocker, s.Source)
		}
	}
	fmt.Fprintln(&b, "\nRanking by eligible static sites (R1 value rows only; references give no kernel gain):")
	for _, stage := range []string{"statements", "lexer"} {
		type rank struct {
			rule string
			n    int
		}
		ranks := []rank{{rule: "R1"}, {rule: "R2"}, {rule: "R3"}}
		for _, s := range sites {
			if !strings.HasPrefix(s.Stage, stage) || s.Verdict != "qualifies" || s.Rule == "R1" && s.Row != "value" {
				continue
			}
			for i := range ranks {
				if ranks[i].rule == s.Rule {
					ranks[i].n++
				}
			}
		}
		sort.SliceStable(ranks, func(i, j int) bool { return ranks[i].n > ranks[j].n })
		sep1, sep2 := " > ", " > "
		if ranks[0].n == ranks[1].n {
			sep1 = " = "
		}
		if ranks[1].n == ranks[2].n {
			sep2 = " = "
		}
		fmt.Fprintf(&b, "%s: %s (%d)%s%s (%d)%s%s (%d)\n", stage, ranks[0].rule, ranks[0].n, sep1, ranks[1].rule, ranks[1].n, sep2, ranks[2].rule, ranks[2].n)
		for i := range ranks {
			ranks[i].n = 0
		}
		for _, s := range sites {
			if strings.HasPrefix(s.Stage, stage) {
				for i := range ranks {
					if ranks[i].rule == s.Rule {
						ranks[i].n++
					}
				}
			}
		}
		sort.SliceStable(ranks, func(i, j int) bool { return ranks[i].n > ranks[j].n })
		fmt.Fprintf(&b, "%s investigation order by screened sites: %s (%d), %s (%d), %s (%d); not a savings estimate\n", stage, ranks[0].rule, ranks[0].n, ranks[1].rule, ranks[1].n, ranks[2].rule, ranks[2].n)
	}
	return b.String()
}
