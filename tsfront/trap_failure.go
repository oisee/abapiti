package tsfront

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func failureBody(n hir.Node, message string) *hir.Stmt {
	return hir.B(&hir.Stmt{Node: n, Kind: hir.Trap, Name: "lowering trap: " + message})
}

// Consume only this body's diagnostics: nested closures have already consumed
// their own failures. Notes preserve the original diagnostic and location.
func (l *lowerer) trapBodyDiagnostics(n *ast.Node, m *hir.Method, start int) {
	if !l.integerOptions.TrapOnFailure {
		return
	}
	messages := append([]string{}, l.failedSignatures[m]...)
	for i := start; i < len(l.diags); i++ {
		d := &l.diags[i]
		if !strings.HasPrefix(d.Category, "note-") {
			messages = append(messages, d.String())
			d.Category = "note-trap-failure"
		}
	}
	if len(messages) > 0 {
		m.Body = failureBody(l.node(n), strings.Join(messages, "; "))
	}
}

// Retain operand IDs and source locations for late diagnostics and backend
// errors. Synthetic ID-zero operands are attributed only by VerifyBody.
func bodyIDs(s *hir.Stmt) map[int]bool { ids, _ := bodyNodes(s); return ids }
func bodyNodes(s *hir.Stmt) (map[int]bool, map[string]bool) {
	ids := map[int]bool{}
	sources := map[string]bool{}
	var visit func(reflect.Value)
	visit = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
			if !v.IsNil() {
				visit(v.Elem())
			}
			return
		}
		if v.Type() == reflect.TypeOf(hir.Node{}) {
			ids[int(v.FieldByName("ID").Int())] = true
			sources[v.FieldByName("Source").String()] = true
			return
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				visit(v.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i))
			}
		}
	}
	visit(reflect.ValueOf(s))
	return ids, sources
}

// Declarations with invalid signatures are removed, not guessed. Subsequent
// verification traps bodies using those members. Constructors keep a trapping
// zero-argument stub, so a dropped constructor can never become a silent new().
func (l *lowerer) repairFailedBodies() {
	// Late numeric diagnostics and module-value diagnostics are not emitted
	// inside a body's boundary. Trap their retained uses before consuming them.
	for _, d := range l.diags {
		if strings.HasPrefix(d.Category, "note-") {
			continue
		}
		for _, c := range l.out.Classes {
			for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
				if m != nil {
					_, sources := bodyNodes(m.Body)
					if sources[d.Loc] {
						m.Body = failureBody(m.Node, d.String())
					}
				}
			}
			for i := len(c.Fields) - 1; i >= 0; i-- {
				if c.Fields[i].Source == d.Loc {
					c.Fields = append(c.Fields[:i], c.Fields[i+1:]...)
				}
			}
		}
	}
	type entry struct {
		c    *hir.Class
		m    *hir.Method
		body *hir.Stmt
	}
	var entries []entry
	initializationFields := map[*hir.Method]map[string]bool{}
	for _, c := range l.out.Classes {
		for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if m != nil {
				if isTestInitialization(m) {
					fields := map[string]bool{}
					var assignments func(*hir.Stmt)
					assignments = func(s *hir.Stmt) {
						if s == nil {
							return
						}
						if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.StaticGet && s.X.Owner == c.Name {
							fields[s.X.Name] = true
						}
						for _, child := range s.List {
							assignments(child)
						}
					}
					assignments(m.Body)
					initializationFields[m] = fields
				}
				entries = append(entries, entry{c, m, m.Body})
				if !m.Abstract {
					m.Body = failureBody(m.Node, "signature verification placeholder")
				}
			}
		}
	}
	for round := 0; round < 10; round++ {
		errors := hir.Verify(l.out)
		if len(errors) == 0 {
			break
		}
		changed := false
		for _, err := range errors {
			e, ok := err.(hir.Error)
			if !ok {
				continue
			}
			for _, c := range l.out.Classes {
				for i := len(c.Fields) - 1; i >= 0; i-- {
					if c.Fields[i].ID == e.ID {
						c.Fields = append(c.Fields[:i], c.Fields[i+1:]...)
						changed = true
					}
				}
				for i := len(c.Methods) - 1; i >= 0; i-- {
					if c.Methods[i].ID == e.ID {
						c.Methods = append(c.Methods[:i], c.Methods[i+1:]...)
						changed = true
					}
				}
				if c.Ctor != nil && c.Ctor.ID == e.ID {
					c.Ctor.Params = nil
					c.Ctor.Result = hir.T(hir.Void)
					for j := range entries {
						if entries[j].m == c.Ctor {
							entries[j].body = failureBody(c.Ctor.Node, e.Error())
						}
					}
					changed = true
				}
			}
			for _, in := range l.out.Interfaces {
				for i := len(in.Methods) - 1; i >= 0; i-- {
					if in.Methods[i].ID == e.ID {
						in.Methods = append(in.Methods[:i], in.Methods[i+1:]...)
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	for _, e := range entries {
		e.m.Body = e.body
	}
	// A failed static initialization must not expose fabricated zero values.
	for round := 0; round < len(entries)+1; round++ {
		changed := false
		for _, c := range l.out.Classes {
			for _, m := range c.Methods {
				if (m.Name == "class_constructor" || isTestInitialization(m)) && isFailureBody(m.Body) {
					var retained []hir.Field
					for _, f := range c.Fields {
						if !f.Static || (isTestInitialization(m) && !initializationFields[m][f.Name]) {
							retained = append(retained, f)
						}
					}
					if len(retained) != len(c.Fields) {
						changed = true
					}
					c.Fields = retained
				}
			}
		}
		for _, entry := range entries {
			m := entry.m
			if errors := hir.VerifyBody(l.out, entry.c, m); len(errors) > 0 {
				m.Body = failureBody(m.Node, fmt.Sprintf("%s: HIR verification: %s", m.Source, errors[0]))
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	// Unsupported declarations (e.g. external namespaces) have no executable
	// body. Uses either diagnosed during lowering or fail verification above.
	for i := range l.diags {
		if !strings.HasPrefix(l.diags[i].Category, "note-") {
			l.diags[i].Message = l.diags[i].String()
			l.diags[i].Category = "note-dropped-declaration"
		}
	}
}
func isFailureBody(s *hir.Stmt) bool {
	return s != nil && s.Kind == hir.Block && len(s.List) == 1 && s.List[0].Kind == hir.Trap
}
