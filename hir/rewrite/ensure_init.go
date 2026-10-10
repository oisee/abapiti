package rewrite

import (
	"fmt"
	"math"
	"strconv"

	"github.com/oisee/abapiti/hir"
)

// FieldKey keeps the declaring owner and member name separate; concatenated
// names are ambiguous when either component contains punctuation.
type FieldKey struct{ Owner, Name string }

// Mirror hir/abap/constants.go: a single top-level literal assignment in the
// static initializer becomes a constant only when no other write exists.
// Keep this adapter-only: EnsureInit describes emitted guards, not a HIR node.
func (x *extractor) findInitializers(p *hir.Program) {
	x.constants = map[FieldKey]bool{}
	x.initializers = map[string]bool{}
	seen := map[FieldKey]int{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if m.Name == "class_constructor" && m.Static && m.Body != nil {
				for _, s := range m.Body.List {
					if constantInitializer(c, s) {
						x.constants[FieldKey{c.Name, s.X.Name}] = true
					}
				}
			}
		}
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			visitTree(m.Body, "", func(s *hir.Stmt, _ string) {
				if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.StaticGet {
					seen[FieldKey{s.X.Owner, s.X.Name}]++
				}
			}, func(*hir.Expr, string) {})
		}
	}
	for k := range x.constants {
		if seen[k] != 1 {
			delete(x.constants, k)
		}
	}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if m.Name != "class_constructor" || !m.Static || m.Body == nil {
				continue
			}
			for _, s := range m.Body.List {
				if s == nil || s.Kind != hir.Assign || s.X == nil || s.X.Kind != hir.StaticGet || s.X.Owner != c.Name || !x.constants[FieldKey{c.Name, s.X.Name}] {
					x.initializers[c.Name] = true
				}
			}
		}
	}
}

func constantInitializer(c *hir.Class, s *hir.Stmt) bool {
	if s == nil || s.Kind != hir.Assign || s.X == nil || s.X.Kind != hir.StaticGet || s.X.Owner != c.Name || s.Y == nil || s.Y.Kind != hir.Lit || s.Y.Value == nil {
		return false
	}
	var field *hir.Field
	for i := range c.Fields {
		if c.Fields[i].Name == s.X.Name && c.Fields[i].Static {
			field = &c.Fields[i]
		}
	}
	if field == nil || !field.Type.Equal(s.Y.Type) {
		return false
	}
	switch field.Type.Kind {
	case hir.I32, hir.I64, hir.Number:
		v, err := strconv.ParseFloat(fmt.Sprint(s.Y.Value), 64)
		return err == nil && v == math.Trunc(v) && math.Abs(v) <= 1<<53 && (field.Type.Kind != hir.I32 || v >= math.MinInt32 && v <= math.MaxInt32)
	case hir.Bool:
		_, ok := s.Y.Value.(bool)
		return ok
	case hir.String:
		v, ok := s.Y.Value.(string)
		if !ok || len(v) > 120 {
			return false
		}
		for _, r := range v {
			if r < 32 || r > 126 {
				return false
			}
		}
		return true
	}
	return false
}
