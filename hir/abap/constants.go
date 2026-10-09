package abap

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// findStaticConstants marks static fields that the class initializer sets to
// a primitive literal and that nothing else in the program ever writes. They
// are emitted as CONSTANTS: no lazy initialization, no guard on reads.
func (e *emitter) findStaticConstants() {
	e.staticConsts = map[string]string{}
	e.staticInit = map[string]bool{}
	for _, c := range e.p.Classes {
		for _, m := range c.Methods {
			if m.Name != "class_constructor" || !m.Static || m.Body == nil {
				continue
			}
			for _, s := range m.Body.List {
				if lit, ok := constantInit(c, s); ok {
					e.staticConsts[c.Name+"."+s.X.Name] = lit
				}
			}
		}
	}
	// Any other write disqualifies the field.
	seen := map[string]int{}
	var stmt func(*hir.Stmt)
	var expr func(*hir.Expr)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.StaticGet {
			seen[s.X.Owner+"."+s.X.Name]++
		}
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, c := range s.List {
			stmt(c)
		}
	}
	for _, c := range e.p.Classes {
		if c.Ctor != nil {
			stmt(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			stmt(m.Body)
		}
	}
	for key := range e.staticConsts {
		if seen[key] != 1 {
			delete(e.staticConsts, key)
		}
	}
	for _, c := range e.p.Classes {
		for _, m := range c.Methods {
			if m.Name != "class_constructor" || !m.Static || m.Body == nil {
				continue
			}
			for _, s := range m.Body.List {
				if !e.isStaticConstantInit(c, s) {
					e.staticInit[c.Name] = true
				}
			}
		}
	}
}

func (e *emitter) staticConstant(owner, field string) (string, bool) {
	lit, ok := e.staticConsts[owner+"."+field]
	return lit, ok
}

func (e *emitter) isStaticConstantInit(c *hir.Class, s *hir.Stmt) bool {
	if s == nil || s.Kind != hir.Assign || s.X == nil || s.X.Kind != hir.StaticGet || s.X.Owner != c.Name {
		return false
	}
	_, ok := e.staticConstant(c.Name, s.X.Name)
	return ok
}

// constantInit recognises `Class.field = <primitive literal>` in a class
// initializer and renders the literal as an ABAP CONSTANTS value.
func constantInit(c *hir.Class, s *hir.Stmt) (string, bool) {
	if s == nil || s.Kind != hir.Assign || s.X == nil || s.X.Kind != hir.StaticGet || s.X.Owner != c.Name || s.Y == nil || s.Y.Kind != hir.Lit || s.Y.Value == nil {
		return "", false
	}
	var field *hir.Field
	for i := range c.Fields {
		if c.Fields[i].Name == s.X.Name && c.Fields[i].Static {
			field = &c.Fields[i]
		}
	}
	if field == nil || !field.Type.Equal(s.Y.Type) {
		return "", false
	}
	switch field.Type.Kind {
	case hir.I32, hir.I64, hir.Number:
		v, err := strconv.ParseFloat(fmt.Sprint(s.Y.Value), 64)
		if err != nil || v != math.Trunc(v) || math.Abs(v) > 1<<53 {
			return "", false
		}
		if field.Type.Kind == hir.I32 && (v < math.MinInt32 || v > math.MaxInt32) {
			return "", false
		}
		if field.Type.Kind == hir.Number {
			return "'" + strconv.FormatInt(int64(v), 10) + "'", true
		}
		return strconv.FormatInt(int64(v), 10), true
	case hir.Bool:
		if b, ok := s.Y.Value.(bool); ok {
			if b {
				return "abap_true", true
			}
			return "abap_false", true
		}
	case hir.String:
		v, ok := s.Y.Value.(string)
		if !ok || len(v) > 120 {
			return "", false
		}
		for _, r := range v {
			if r < 32 || r > 126 {
				return "", false
			}
		}
		return "`" + strings.ReplaceAll(v, "`", "``") + "`", true
	}
	return "", false
}
