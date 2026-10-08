package tsfront

import (
	"fmt"
	"math"

	"github.com/oisee/abapiti/hir"
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
				terms, ok := indexFieldTerms(s.Y)
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
						if param.Name == term.param && param.Type.Kind == hir.Number && !m.Internal {
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
				if indexParamAssigned(m.Body, param.Name) {
					continue
				}
				originalBody := m.Body
				walkNumberStmt(originalBody, func(s *hir.Stmt) {}, func(e *hir.Expr) {
					if e.Kind == hir.Local && e.Name == param.Name {
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
	offset int64
}

func indexFieldTerms(e *hir.Expr) ([]indexFieldTerm, bool) {
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
		return []indexFieldTerm{{param: e.Name}}, true
	case hir.Conditional:
		a, ok := indexFieldTerms(e.Y)
		if !ok {
			return nil, false
		}
		b, ok := indexFieldTerms(e.Z)
		return append(a, b...), ok
	case hir.Binary:
		if e.Op != "+" && e.Op != "-" {
			return nil, false
		}
		left, ok := indexFieldTerms(e.X)
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

func indexParamAssigned(body *hir.Stmt, name string) bool {
	assigned := false
	walkNumberStmt(body, func(s *hir.Stmt) {
		if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && s.X.Name == name {
			assigned = true
		}
	}, func(e *hir.Expr) {})
	return assigned
}
