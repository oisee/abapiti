package tsfront

import (
	"fmt"
	"math"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Public number parameters normally remain unconstrained binary64. The task's
// checked-boundary policy applies narrowly to parameters stored in private
// string-index fields: validate an exact integer before specializing storage.
// Public signatures stay Number; source-located diagnostics expose the contract.
// No class or member names participate in selecting this optimization.
func (l *lowerer) checkIndexFieldBoundaries() {
	for _, c := range l.out.Classes {
		indexFields := map[string]bool{}
		private := map[string]bool{}
		for _, f := range c.Fields {
			private[f.Name] = f.Private && f.Type.Kind == hir.Number
		}
		for _, m := range numberMethods(c) {
			walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
				if e.Kind == hir.RuntimeOp && e.Op == "number.index" && e.X != nil && e.X.Kind == hir.FieldGet && e.X.X != nil && e.X.X.Kind == hir.This && private[e.X.Name] {
					indexFields[e.X.Name] = true
				}
			})
		}
		if len(indexFields) == 0 {
			continue
		}
		// Each eligible field must have only literal writes or affine writes from
		// a single public parameter. Ambiguous writers keep the original contract.
		type boundary struct {
			m      *hir.Method
			name   string
			lo, hi int64
		}
		candidates := map[string]*boundary{}
		bad := map[string]bool{}
		for _, m := range numberMethods(c) {
			walkNumberStmt(m.Body, func(s *hir.Stmt) {
				if s.Kind != hir.Assign || s.X == nil || s.X.Kind != hir.FieldGet || s.X.X == nil || s.X.X.Kind != hir.This || !indexFields[s.X.Name] {
					return
				}
				terms, ok := l.indexFieldTerms(s.Y)
				if !ok {
					bad[s.X.Name] = true
					return
				}
				for _, term := range terms {
					if term.param == "" {
						if term.offset < math.MinInt32 || term.offset > math.MaxInt32 {
							bad[s.X.Name] = true
						}
						continue
					}
					numericParam := false
					for _, param := range m.Params {
						if param.Name == term.param && term.symbol != nil && l.paramSymbols[m][param.Name] == term.symbol && param.Type.Kind == hir.Number && !m.Internal {
							numericParam = true
						}
					}
					if !numericParam {
						bad[s.X.Name] = true
						continue
					}
					old := candidates[s.X.Name]
					if old != nil && (old.m != m || old.name != term.param) {
						bad[s.X.Name] = true
						continue
					}
					if old == nil {
						old = &boundary{m: m, name: term.param, lo: math.MinInt32, hi: math.MaxInt32}
						candidates[s.X.Name] = old
					}
					old.lo = max(old.lo, int64(math.MinInt32)-term.offset)
					old.hi = min(old.hi, int64(math.MaxInt32)-term.offset)
				}
			}, func(e *hir.Expr) {})
		}
		// Intersect the requirements of every field fed by the same parameter.
		byMethod := map[*hir.Method]map[string]*boundary{}
		for field, b := range candidates {
			if bad[field] {
				continue
			}
			if byMethod[b.m] == nil {
				byMethod[b.m] = map[string]*boundary{}
			}
			old := byMethod[b.m][b.name]
			if old == nil {
				copy := *b
				byMethod[b.m][b.name] = &copy
			} else {
				old.lo = max(old.lo, b.lo)
				old.hi = min(old.hi, b.hi)
			}
		}
		for _, m := range numberMethods(c) {
			bounds := byMethod[m]
			if len(bounds) == 0 || m.Body == nil {
				continue
			}
			for i, param := range m.Params {
				b := bounds[param.Name]
				if b == nil || b.lo > b.hi {
					continue
				}
				l.serial++
				alias := fmt.Sprintf("range_param_%d", l.serial)
				used := map[string]bool{}
				for _, existing := range m.Params {
					used[existing.Name] = true
				}
				walkNumberStmt(m.Body, func(s *hir.Stmt) { used[s.Name] = true }, func(e *hir.Expr) {
					if e.Kind == hir.Local {
						used[e.Name] = true
					}
				})
				for used[alias] {
					l.serial++
					alias = fmt.Sprintf("range_param_%d", l.serial)
				}
				if l.indexParamAssigned(m.Body, l.paramSymbols[m][param.Name]) {
					continue
				}
				originalBody := m.Body
				walkNumberStmt(originalBody, func(s *hir.Stmt) {}, func(e *hir.Expr) {
					if e.Kind == hir.Local && l.localSymbols[e] != nil && l.localSymbols[e] == l.paramSymbols[m][param.Name] {
						e.Name = alias
					}
				})
				proof := &hir.IntegerRange{Min: b.lo, Max: b.hi}
				checked := &hir.Expr{Node: m.Node, Kind: hir.CheckedNumericConvert, Type: hir.T(hir.I32), Range: proof, X: hir.V(param.Name, hir.T(hir.Number))}
				// Keep the original lexical block. The checked alias dominates every use.
				m.Body = hir.B(&hir.Stmt{Node: m.Node, Kind: hir.VarDecl, Name: alias, Type: hir.T(hir.I32), X: checked}, originalBody)
				l.diags = append(l.diags, LowerDiagnostic{Category: "note-number-boundary", Loc: m.Source, Message: fmt.Sprintf("%s.%s parameter %d (%s): checked integer boundary [%d,%d] for private string-index storage; other values raise cx_sy_range_out_of_bounds", c.Name, m.Name, i, param.Name, b.lo, b.hi)})
			}
		}
	}
}

type indexFieldTerm struct {
	param  string
	symbol *ast.Symbol
	offset int64
}

func (l *lowerer) indexFieldTerms(e *hir.Expr) ([]indexFieldTerm, bool) {
	e = unwrapNumber(e)
	if e == nil {
		return nil, false
	}
	switch e.Kind {
	case hir.Lit:
		r := numericLiteral(e)
		if r.state == 1 {
			return []indexFieldTerm{{offset: r.lo}}, true
		}
	case hir.Local:
		return []indexFieldTerm{{param: e.Name, symbol: l.localSymbols[e]}}, true
	case hir.Conditional:
		a, ok := l.indexFieldTerms(e.Y)
		if !ok {
			return nil, false
		}
		b, ok := l.indexFieldTerms(e.Z)
		return append(a, b...), ok
	case hir.Binary:
		if e.Op != "+" && e.Op != "-" {
			return nil, false
		}
		left, ok := l.indexFieldTerms(e.X)
		right := numericLiteral(e.Y)
		if !ok || right.state != 1 {
			return nil, false
		}
		delta := right.lo
		if e.Op == "-" {
			delta = -delta
		}
		for i := range left {
			left[i].offset += delta
			if left[i].offset < -safeInteger || left[i].offset > safeInteger {
				return nil, false
			}
		}
		return left, true
	}
	return nil, false
}

func (l *lowerer) indexParamAssigned(body *hir.Stmt, symbol *ast.Symbol) bool {
	assigned := false
	walkNumberStmt(body, func(s *hir.Stmt) {
		if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && symbol != nil && l.localSymbols[s.X] == symbol {
			assigned = true
		}
	}, func(e *hir.Expr) {})
	return assigned
}

// HIR requires distinct local names. Normalize source shadowing by checker
// binding before name-keyed range analysis, leaving synthetic locals alone.
func (l *lowerer) distinguishNumberLocals() {
	for _, c := range l.out.Classes {
		for _, m := range numberMethods(c) {
			used := map[string]bool{}
			for _, param := range m.Params {
				used[param.Name] = true
			}
			walkNumberStmt(m.Body, func(s *hir.Stmt) {
				if s.Name != "" {
					used[s.Name] = true
				}
			}, func(e *hir.Expr) {
				if e.Kind == hir.Local {
					used[e.Name] = true
				}
			})
			seen := map[string]bool{}
			for _, param := range m.Params {
				seen[param.Name] = true
			}
			renamed := map[*ast.Symbol]string{}
			originalNames := map[*hir.Stmt]string{}
			walkNumberStmt(m.Body, func(s *hir.Stmt) {
				if s.Kind != hir.VarDecl {
					return
				}
				name := s.Name
				originalNames[s] = name
				symbol := l.declSymbols[s]
				if seen[name] && symbol != nil {
					for {
						l.serial++
						s.Name = fmt.Sprintf("range_local_%d", l.serial)
						if !used[s.Name] {
							break
						}
					}
					used[s.Name] = true
					renamed[symbol] = s.Name
				}
				seen[name] = true
			}, func(e *hir.Expr) {})
			walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
				if name := renamed[l.localSymbols[e]]; e.Kind == hir.Local && name != "" {
					e.Name = name
				}
			})
			// Lowering also synthesizes local reads (for example lifted capture
			// arguments) without a source symbol. Bind those reads in their HIR scope.
			l.renameSyntheticNumberLocals(m.Body, originalNames, map[string]string{})
		}
	}
}

func (l *lowerer) renameSyntheticNumberLocals(body *hir.Stmt, original map[*hir.Stmt]string, env map[string]string) {
	var expr func(*hir.Expr, map[string]string)
	var stmt func(*hir.Stmt, map[string]string)
	clone := func(env map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range env {
			out[k] = v
		}
		return out
	}
	expr = func(e *hir.Expr, env map[string]string) {
		if e == nil {
			return
		}
		if e.Kind == hir.Local && l.localSymbols[e] == nil {
			if name := env[e.Name]; name != "" {
				e.Name = name
			}
		}
		if e.Kind == hir.Seq {
			scope := clone(env)
			if e.Stmt != nil {
				for _, s := range e.Stmt.List {
					stmt(s, scope)
				}
			}
			expr(e.Y, scope)
			return
		}
		expr(e.X, env)
		expr(e.Y, env)
		expr(e.Z, env)
		for _, a := range e.Args {
			expr(a, env)
		}
	}
	stmt = func(s *hir.Stmt, env map[string]string) {
		if s == nil {
			return
		}
		if s.Kind == hir.Block {
			scope := clone(env)
			for _, child := range s.List {
				stmt(child, scope)
			}
			return
		}
		expr(s.X, env)
		expr(s.Y, env)
		if s.Kind == hir.VarDecl {
			name := original[s]
			if name == "" {
				name = s.Name
			}
			env[name] = s.Name
		}
		if s.Kind == hir.ForEach {
			scope := clone(env)
			scope[s.Name] = s.Name
			stmt(s.Body, scope)
			return
		}
		stmt(s.Body, env)
		stmt(s.Else, env)
		for _, child := range s.List {
			stmt(child, env)
		}
	}
	stmt(body, env)
}
