package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// The LSP debug dump needs native lexer tokens and their Position values. Keep
// this certificate local to that family: ordinary class layout is not an own-
// property proof. Validate every constructor in base-first insertion order;
// parameters are permitted, but this must not escape during initialization.
func (e *emitter) lspTokenJSONOrder(c *hir.Class) ([]string, bool) {
	if c == nil {
		return nil, false
	}
	root := c
	for root.Super != "" {
		root = e.classBy(root.Super)
		if root == nil {
			return nil, false
		}
	}
	if root.Name != "src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken" && root.Name != "src/position.ts.Position" {
		return nil, false
	}
	var chain []*hir.Class
	for p := c; p != nil; p = e.classBy(p.Super) {
		chain = append(chain, p)
	}
	var order []string
	seen := map[string]bool{}
	for i := len(chain) - 1; i >= 0; i-- {
		p := chain[i]
		// ViewUnsafe can be sealed by a structural assertion elsewhere in
		// the closure. Nominal constructor fields do not use that adapter.
		if p.Structural || p.PrototypeAccessors || p.Scoped || p.CoercionUnsafe || p.UndefinedDefineFields || !jsonKeyNamesSafe(p) {
			return nil, false
		}
		for _, m := range p.Methods {
			if !m.Static && (m.Name == "toJSON" || m.Name == "get.toJSON") {
				return nil, false
			}
		}
		var declared, assigned []string
		for _, f := range p.Fields {
			if !f.Static {
				if strings.HasPrefix(f.Name, "#") || f.Name == "__proto__" {
					return nil, false
				}
				if seen[f.Name] {
					return nil, false
				}
				seen[f.Name] = true
				declared = append(declared, f.Name)
			}
		}
		valid, superCalls := true, 0
		var safeValue func(*hir.Expr) bool
		safeValue = func(x *hir.Expr) bool {
			if x == nil {
				return true
			}
			if x.Kind == hir.This || x.Stmt != nil {
				return false
			}
			if !safeValue(x.X) || !safeValue(x.Y) || !safeValue(x.Z) {
				return false
			}
			for _, a := range x.Args {
				if !safeValue(a) {
					return false
				}
			}
			return true
		}
		var visit func(*hir.Stmt)
		visit = func(s *hir.Stmt) {
			if s == nil {
				return
			}
			switch s.Kind {
			case hir.Block:
				for _, child := range s.List {
					visit(child)
				}
			case hir.Assign:
				if s.X == nil || s.X.Kind != hir.FieldGet || s.X.X == nil || s.X.X.Kind != hir.This || !safeValue(s.Y) {
					valid = false
					return
				}
				assigned = append(assigned, s.X.Name)
			case hir.ExprStmt:
				if s.X == nil || s.X.Kind != hir.SuperCall || superCalls != 0 || len(assigned) != 0 || !safeValue(s.X) {
					valid = false
					return
				}
				superCalls++
			default:
				valid = false
			}
		}
		if p.Ctor != nil {
			visit(p.Ctor.Body)
		}
		if p.Super != "" && p.Ctor != nil && superCalls != 1 {
			return nil, false
		}
		if !valid || strings.Join(declared, "\x00") != strings.Join(assigned, "\x00") {
			return nil, false
		}
		order = append(order, declared...)
	}
	return order, true
}
