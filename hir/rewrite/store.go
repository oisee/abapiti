package rewrite

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/oisee/abapiti/grace"
	"github.com/oisee/abapiti/hir"
)

type copyPlan struct {
	defID    string
	def      *hir.Stmt
	rhs      *hir.Expr
	snapshot *hir.Expr
}

// BeforeEmission is shared by both backends. Explicit calls to CopyProp do not
// depend on the environment; emission remains unchanged unless opted in.
func BeforeEmission(p *hir.Program) (Stats, error) {
	if os.Getenv("ABAPITI_COPYPROP") != "1" {
		return Stats{}, nil
	}
	return CopyProp(p)
}

// CopyProp applies the embedded copy propagation and dead-store rules to a
// verified fixed point. Each round strictly reduces stores, within existing bounds.
func CopyProp(p *hir.Program) (Stats, error) {
	b, err := ruleFiles.ReadFile("rules/stores.grace")
	if err != nil {
		return Stats{}, err
	}
	_, rules, err := Parse(string(b))
	if err != nil {
		return Stats{}, err
	}
	return Rewrite(p, rules, Limits{Rounds: 16})
}

// storeInert is a deliberately small proof, rather than absence of an impurity
// fact. In particular it excludes identity allocation, calls, numeric arithmetic
// operations, nullable dereferences and all unreviewed expression kinds.
func storeInert(e *hir.Expr) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case hir.Lit, hir.Local, hir.This:
		return true
	case hir.RuntimeOp:
		effect, ok := RuntimeEffects[e.Op]
		// A non-optional HIR reference can still be an uninitialized local. The
		// operation table does not prove a valid receiver for this expression.
		if e.X != nil && (e.X.Type.IsRef() || e.X.Type.Kind == hir.Optional) {
			return false
		}
		if !ok || effect.Writes != "None" || effect.Allocates || effect.MayRaise != "None" || effect.Reads.Global || effect.Conservative != "" {
			return false
		}
	case hir.IsUndefined, hir.ToBoolean, hir.InstanceOf:
	case hir.Narrow:
		if e.X == nil || !e.Type.Equal(e.X.Type) {
			return false
		}
	case hir.Binary:
		if e.Op == "/" || e.Op == "%" || e.CheckIntegerOverflow || (e.Type.Kind == hir.Number || e.Type.Kind == hir.I32 || e.Type.Kind == hir.I64) && (e.Op == "+" || e.Op == "-" || e.Op == "*") {
			return false
		}
	case hir.Unary:
		if e.CheckIntegerOverflow || e.Op == "-" {
			return false
		}
	case hir.Conditional:
	default:
		return false
	}
	for _, child := range []*hir.Expr{e.X, e.Y, e.Z} {
		if child != nil && !storeInert(child) {
			return false
		}
	}
	for _, child := range e.Args {
		if !storeInert(child) {
			return false
		}
	}
	return e.Stmt == nil
}

func storeRHS(s *hir.Stmt) *hir.Expr {
	if s == nil {
		return nil
	}
	if s.Kind == hir.VarDecl {
		return s.X
	}
	if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local {
		return s.Y
	}
	return nil
}
func dropStore(s *hir.Stmt) {
	if s.Kind == hir.VarDecl {
		s.X = nil
	} else {
		*s = hir.Stmt{Node: s.Node, Kind: hir.Block}
	}
}
func (r *runner) countStore(m *hir.Method, copy bool) {
	id := r.methods[m]
	n := r.stats.Methods[id]
	if copy {
		r.stats.CopySites++
		n[0]++
	} else {
		r.stats.DeadStores++
		n[1]++
	}
	r.stats.Methods[id] = n
}

// Native facts prove syntax, types and evaluation order. Grace selects the
// rewrite and supplies CFG liveness. Restricting motion to one statement list
// excludes loop duplication, branch dominance and try/finally crossings.
func (r *runner) addStoreFacts() error {
	needed := r.rules == nil
	var rewrites []grace.RewriteRule
	if r.rules != nil {
		rewrites = r.rules.Rewrites()
	}
	for _, rule := range rewrites {
		if rule.Action() == "substitute-use" || rule.Action() == "remove-statement" {
			needed = true
		}
	}
	if !needed {
		return nil
	}
	r.copies = map[string]copyPlan{}
	refs := map[string]string{}
	if rows := r.db.Facts("local_ref"); len(rows) > 0 {
		for _, row := range rows {
			if r.localStoreBinding(row[1]) {
				refs[row[0]] = row[1]
			}
		}
	}
	defs := map[string][]string{}
	if rows := r.db.Facts("def"); len(rows) > 0 {
		for _, row := range rows {
			n := r.nodes[row[1]]
			if !r.localStoreBinding(row[0]) {
				continue
			}
			if n.stmt != nil && n.stmt.Kind == hir.VarDecl && n.stmt.X == nil {
				continue
			}
			defs[row[0]] = append(defs[row[0]], row[1])
		}
	}
	uses := map[string][]string{}
	if rows := r.db.Facts("use"); len(rows) > 0 {
		for _, row := range rows {
			if r.localStoreBinding(row[0]) {
				uses[row[0]] = append(uses[row[0]], row[1])
			}
		}
	}
	type position struct {
		block *hir.Stmt
		index int
		path  string
	}
	positions := map[string]position{}
	for path, n := range r.nodes {
		if n.stmt != nil && n.stmt.Kind == hir.Block {
			for i := range n.stmt.List {
				positions[fmt.Sprintf("%s/s%d", path, i)] = position{n.stmt, i, path}
			}
		}
	}
	for id, n := range r.nodes {
		if r.refresh != nil && !r.refresh[n.method] {
			continue
		}
		rhs := storeRHS(n.stmt)
		if rhs == nil {
			continue
		}
		binding := id + "/local/" + n.stmt.Name
		typ := n.stmt.Type
		if n.stmt.Kind == hir.Assign {
			binding = refs[id+"/x"]
			typ = n.stmt.X.Type
		}
		// Parameters and catch/iteration bindings are not local declarations.
		if !r.localStoreBinding(binding) {
			continue
		}
		if storeInert(rhs) && typ.Equal(rhs.Type) {
			if err := r.db.Add("store_inert", id, binding); err != nil {
				return err
			}
		}
		if len(defs[binding]) != 1 || len(uses[binding]) != 1 || !typ.Equal(rhs.Type) {
			continue
		}
		useID := uses[binding][0]
		use := r.nodes[useID].expr
		captured := false
		for ancestor := useID; ancestor != ""; {
			if node := r.nodes[ancestor].expr; node != nil && (node.Kind == hir.New && strings.HasPrefix(node.Owner, "closure.") || (node.Kind == hir.DirectCall || node.Kind == hir.VirtualCall) && strings.HasPrefix(node.Name, "fn_")) {
				captured = true
				break
			}
			slash := strings.LastIndex(ancestor, "/")
			if slash < 0 {
				break
			}
			ancestor = ancestor[:slash]
		}
		if captured {
			continue
		}
		if use == nil || !use.Type.Equal(typ) {
			continue
		}
		pos, ok := positions[id]
		if !ok {
			continue
		}
		for j := pos.index + 1; j < len(pos.block.List); j++ {
			s := pos.block.List[j]
			sid := fmt.Sprintf("%s/s%d", pos.path, j)
			if strings.HasPrefix(useID, sid+"/") && sid != "" {
				if s.Kind != hir.VarDecl && s.Kind != hir.Assign && s.Kind != hir.Return && s.Kind != hir.ExprStmt {
					break
				}
				// Whole-expression use is the next evaluation. All other motion requires
				// total, inert RHS and an inert destination (no earlier alias mutation).
				whole := s.X == use && (s.Kind == hir.Return || s.Kind == hir.VarDecl || s.Kind == hir.ExprStmt) || s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && s.Y == use
				safe := whole && j == pos.index+1
				if storeInert(rhs) && (whole || prefixInert(s.X, use) || s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && prefixInert(s.Y, use)) {
					safe = true
				}
				if safe {
					r.copies[useID] = copyPlan{id, n.stmt, rhs, snapshotExpr(rhs)}
					if err := r.db.Add("copy_proven", useID, id, binding); err != nil {
						return err
					}
				}
				break
			}
			if !motionPast(s, rhs) {
				break
			}
		}
	}
	return r.addStoreEdges()
}

// Moving an inert value past inert stores is safe only if none writes a local
// it reads. Anything with heap/runtime/call effects is a conservative barrier.
func motionPast(s *hir.Stmt, rhs *hir.Expr) bool {
	if s == nil || !storeInert(rhs) {
		return false
	}
	v := storeRHS(s)
	if v == nil || !storeInert(v) {
		return false
	}
	name := s.Name
	if s.Kind == hir.Assign {
		name = s.X.Name
	}
	reads := false
	visitExpr(rhs, "", func(*hir.Stmt, string) {}, func(e *hir.Expr, _ string) {
		if e.Kind == hir.Local && e.Name == name {
			reads = true
		}
	})
	return !reads
}
func (r *runner) removeStore(s *hir.Stmt, m *hir.Method) *hir.Stmt {
	id := r.stmtIDs[s]
	if id == "" || storeRHS(s) == nil || !r.unreadStores[id] {
		return s
	}
	for i, rule := range r.rules.Rewrites() {
		if rule.Action() != "remove-statement" {
			continue
		}
		if len(r.plans[i].Match(Tuple{id, string(s.Kind)})) > 0 {
			// Recheck RHS after earlier substitutions in the same round.
			if !storeInert(storeRHS(s)) {
				continue
			}
			dropStore(s)
			r.countStore(m, false)
			r.changed++
			r.dirty[r.methods[m]] = true
			return s
		}
	}
	return s
}

// StoreReport is stable and useful to measurement tools.
func StoreReport(s Stats) string {
	return fmt.Sprintf("copyprop=%d dse=%d rounds=%d", s.CopySites, s.DeadStores, s.Rounds)
}

// prefixInert proves that evaluations preceding this use in its expression
// cannot write its dependencies. The containing operation executes afterwards.
func prefixInert(e, use *hir.Expr) bool {
	if e == nil {
		return false
	}
	if e == use {
		return true
	}
	if e.Stmt != nil {
		return false
	}
	children := append([]*hir.Expr{e.X, e.Y, e.Z}, e.Args...)
	for _, child := range children {
		if child == nil {
			continue
		}
		if prefixInert(child, use) {
			return true
		}
		if !storeInert(child) {
			return false
		}
	}
	return false
}

// A preceding rewrite can change the RHS of another planned copy. Such a
// candidate is deferred to the next verified round, where its guards are fresh.
func (p copyPlan) unchanged() bool {
	return storeRHS(p.def) == p.rhs && reflect.DeepEqual(p.rhs, p.snapshot)
}
func snapshotExpr(e *hir.Expr) *hir.Expr {
	if e == nil {
		return nil
	}
	out := *e
	out.X = snapshotExpr(e.X)
	out.Y = snapshotExpr(e.Y)
	out.Z = snapshotExpr(e.Z)
	if e.Args != nil {
		out.Args = make([]*hir.Expr, len(e.Args))
		for i, v := range e.Args {
			out.Args[i] = snapshotExpr(v)
		}
	}
	out.Stmt = snapshotStmt(e.Stmt)
	return &out
}
func snapshotStmt(s *hir.Stmt) *hir.Stmt {
	if s == nil {
		return nil
	}
	out := *s
	out.X = snapshotExpr(s.X)
	out.Y = snapshotExpr(s.Y)
	out.Body = snapshotStmt(s.Body)
	out.Else = snapshotStmt(s.Else)
	if s.List != nil {
		out.List = make([]*hir.Stmt, len(s.List))
		for i, v := range s.List {
			out.List[i] = snapshotStmt(v)
		}
	}
	return &out
}

// The fixed store rules do not use hierarchy, escape, or whole-method purity
// facts. Extract only their evaluation-order CFG, rather than unrelated facts.
func extractStoreFacts(p *hir.Program, demanded map[string]bool, active map[*hir.Method]bool) *DB {
	db := NewDB()
	if demanded != nil {
		demanded["next"] = true
	}
	db.SetDemand(demanded)
	x := &extractor{db: db}
	for _, c := range p.Classes {
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			if m.Body == nil || active != nil && !active[m] {
				continue
			}
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			x.method = methodID(c.Name, name)
			env := scope{}
			if !m.Static {
				env["this"] = binding{id: x.method + "/this"}
			}
			for _, param := range m.Params {
				env[param.Name] = binding{id: x.method + "/param/" + param.Name}
			}
			x.flowFactsMode(m, env, true)
		}
	}
	return db
}
func isStoreRules(rules *Rules) bool {
	b, err := ruleFiles.ReadFile("rules/stores.grace")
	if err != nil {
		return false
	}
	_, embedded, err := Parse(string(b))
	return err == nil && reflect.DeepEqual(rules, embedded)
}

// Contract the already-extracted CFG separately for each candidate binding.
// The first read/definition on every path is the exact liveness boundary; this
// avoids materializing liveness at every unrelated local's evaluation point.
func (r *runner) addStoreEdges() error {
	r.unreadStores = map[string]bool{}
	edges := map[string][]string{}
	if rows := r.db.Facts("next"); len(rows) > 0 {
		for _, row := range rows {
			edges[row[0]] = append(edges[row[0]], row[1])
		}
	}
	inert := r.db.Facts("store_inert")
	for _, store := range inert {
		site, v := store[0], store[1]
		r.unreadStores[site] = true
		seen := map[string]bool{}
		pending := append([]string{}, edges[site]...)
		for len(pending) > 0 {
			n := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[n] {
				continue
			}
			seen[n] = true
			read := r.db.Has("use", v, n)
			if read {
				r.unreadStores[site] = false
			}
			if read || r.db.Has("def", v, n) {
				if err := r.db.Add("store_next", v, site, n); err != nil {
					return err
				}
			} else {
				pending = append(pending, edges[n]...)
			}
		}
	}
	return nil
}

// AST pointer sharing has no runtime meaning, but path-indexed actions must
// distinguish each occurrence. Detach only methods with shared mutable syntax;
// ordinary frontend trees and shared immutable literals need no copying.
func detachStoreSyntax(p *hir.Program, rules *Rules) {
	hasStores := false
	for _, rule := range rules.Rewrites() {
		if rule.Action() == "substitute-use" || rule.Action() == "remove-statement" {
			hasStores = true
		}
	}
	if !hasStores {
		return
	}
	// A Method may also be reused as syntax by multiple owning classes.
	owners := map[*hir.Method]bool{}
	unique := func(m *hir.Method) *hir.Method {
		if m == nil {
			return nil
		}
		if owners[m] {
			copy := *m
			m = &copy
		}
		owners[m] = true
		return m
	}
	for _, c := range p.Classes {
		for i, m := range c.Methods {
			c.Methods[i] = unique(m)
		}
		c.Ctor = unique(c.Ctor)
	}
	statements := map[*hir.Stmt]*hir.Method{}
	locals := map[*hir.Expr]*hir.Method{}
	shared := map[*hir.Method]bool{}
	for _, c := range p.Classes {
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			visitTree(m.Body, "", func(s *hir.Stmt, _ string) {
				if prev := statements[s]; prev != nil {
					shared[prev] = true
					shared[m] = true
				} else {
					statements[s] = m
				}
			}, func(e *hir.Expr, _ string) {
				if e.Kind != hir.Local {
					return
				}
				if prev := locals[e]; prev != nil {
					shared[prev] = true
					shared[m] = true
				} else {
					locals[e] = m
				}
			})
		}
	}
	for m := range shared {
		m.Body = snapshotStmt(m.Body)
	}
}

// A method needs an inert store or a possible immediate whole-expression copy.
// Other effectful stores cannot select either embedded action. This is only a
// work filter; all guards still run on complete eligible methods.
func storeCandidateMethods(p *hir.Program) map[*hir.Method]bool {
	active := map[*hir.Method]bool{}
	for _, c := range p.Classes {
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			walkSyntax(m.Body, func(s *hir.Stmt, _ string) {
				if rhs := storeRHS(s); rhs != nil && storeType(s).Equal(rhs.Type) && storeInert(rhs) {
					active[m] = true
				}
				if s.Kind != hir.Block {
					return
				}
				for i := 0; i+1 < len(s.List); i++ {
					def, use := s.List[i], s.List[i+1]
					rhs := storeRHS(def)
					if rhs == nil || !storeType(def).Equal(rhs.Type) {
						continue
					}
					name := def.Name
					if def.Kind == hir.Assign {
						name = def.X.Name
					}
					var value *hir.Expr
					switch use.Kind {
					case hir.VarDecl, hir.Return, hir.ExprStmt:
						value = use.X
					case hir.Assign:
						if use.X != nil && use.X.Kind == hir.Local {
							value = use.Y
						}
					}
					if value != nil && value.Kind == hir.Local && value.Name == name {
						active[m] = true
					}
				}
			}, func(*hir.Expr, string) {})
		}
	}
	return active
}

func storeType(s *hir.Stmt) hir.Type {
	if s.Kind == hir.Assign {
		return s.X.Type
	}
	return s.Type
}

// A local binding must point to its actual VarDecl, rather than a marker in an
// owning source path. This also works for read-only extraction of shared syntax.
func (r *runner) localStoreBinding(binding string) bool {
	marker := strings.LastIndex(binding, "/local/")
	if marker < 0 {
		return false
	}
	decl := r.nodes[binding[:marker]].stmt
	return decl != nil && decl.Kind == hir.VarDecl && decl.Name == binding[marker+len("/local/"):]
}
