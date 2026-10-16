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
	l.repairInterfaceClaims()
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

// Signature repair can drop an implementation while retaining its valid
// interface contract. Missing members become located refusals. Incompatible
// existing ABIs are never changed: drop that interface claim so VerifyBody
// below traps dependent conversions instead of emitting invalid Go.
func (l *lowerer) repairInterfaceClaims() {
	interfaces := map[string]*hir.Interface{}
	for _, i := range l.out.Interfaces {
		interfaces[i.Name] = i
	}
	for _, c := range l.out.Classes {
		methods := map[string]*hir.Method{}
		fields := map[string]bool{}
		for b := c; b != nil; b = l.classByName(b.Super) {
			for _, m := range b.Methods {
				if methods[m.Name] == nil {
					methods[m.Name] = m
				}
			}
			for _, f := range b.Fields {
				fields[f.Name] = true
			}
		}
		var retained []string
		for _, name := range c.Implements {
			i := interfaces[name]
			if i == nil {
				retained = append(retained, name)
				continue
			}
			valid := true
			for _, contract := range i.Methods {
				m := methods[contract.Name]
				if m == nil && !fields[contract.Name] {
					node := hir.Node{ID: l.nextID(), Source: c.Source}
					m = &hir.Method{Node: node, Name: contract.Name, Params: append([]hir.Param(nil), contract.Params...), Result: contract.Result, Virtual: true,
						Body: failureBody(node, c.Source+": missing interface implementation "+contract.Name)}
					c.Methods = append(c.Methods, m)
					methods[m.Name] = m
				}
				compatible := m != nil && !m.Static && (!m.Abstract || c.Abstract) && m.Result.Equal(contract.Result) && len(m.Params) == len(contract.Params)
				if compatible {
					for j, p := range m.Params {
						if p.Name != contract.Params[j].Name || !p.Type.Equal(contract.Params[j].Type) {
							compatible = false
							break
						}
					}
				}
				valid = valid && compatible
			}
			if valid {
				retained = append(retained, name)
			}
		}
		c.Implements = retained
	}
}
