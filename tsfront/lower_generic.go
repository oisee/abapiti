package tsfront

import (
	"github.com/oisee/abapiti/hir"
)

// eraseGenericOverrides fixes the ABI before bodies are lowered. A specialized
// implementation keeps its checker signature behind a forwarding bridge; the
// virtual slot and interface slot retain the declaration's erased signature.
func (l *lowerer) eraseGenericOverrides() {
	classes := map[string]*hir.Class{}
	generic := map[string]bool{}
	for sym, c := range l.classes {
		classes[c.Name] = c
		for _, d := range sym.Declarations {
			if len(d.TypeParameters()) > 0 {
				generic[c.Name] = true
			}
		}
	}
	var visit func(*hir.Class)
	done := map[string]bool{}
	visit = func(c *hir.Class) {
		if c == nil || done[c.Name] {
			return
		}
		done[c.Name] = true
		base := classes[c.Super]
		visit(base)
		if base != nil && generic[base.Name] {
			generic[c.Name] = true
		}
		slots := map[string]*hir.Method{}
		for _, name := range c.Implements {
			for _, i := range l.out.Interfaces {
				if i.Name == name {
					for _, m := range i.Methods {
						slots[m.Name] = m
					}
				}
			}
		}
		if base != nil && generic[c.Name] {
			for _, m := range base.Methods {
				if m.Virtual {
					slots[m.Name] = m
				}
			}
		}
		methods := append([]*hir.Method(nil), c.Methods...)
		for _, m := range methods {
			slot := slots[m.Name]
			if slot == nil || m.Static || len(slot.Params) < len(m.Params) {
				continue
			}
			same := m.Result.Equal(slot.Result) && len(slot.Params) == len(m.Params)
			for j := range m.Params {
				same = same && m.Params[j].Type.Equal(slot.Params[j].Type) && m.Params[j].Name == slot.Params[j].Name
			}
			if same {
				continue
			}
			if m.Abstract {
				for j := range m.Params {
					m.Params[j].Type = slot.Params[j].Type
				}
				m.Result = slot.Result
				continue
			}
			name := m.Name
			m.Name += "_instantiated_" + c.Name
			bridge := &hir.Method{Node: m.Node, Name: name, Virtual: true, Result: slot.Result}
			args := []*hir.Expr{}
			for j, p := range slot.Params {
				bridge.Params = append(bridge.Params, p)
				if j >= len(m.Params) {
					continue
				}
				actual := m.Params[j]
				x := hir.V(p.Name, p.Type)
				if !p.Type.Equal(actual.Type) && !(actual.Type.Kind == hir.ClassRef && actual.Type.Name == hir.RootObject) {
					x = &hir.Expr{Kind: hir.Narrow, Type: actual.Type, X: x}
				}
				args = append(args, x)
			}
			call := &hir.Expr{Kind: hir.VirtualCall, Name: m.Name, Type: m.Result, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}, Args: args}
			if slot.Result.Kind == hir.Void {
				bridge.Body = hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: call})
			} else {
				bridge.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: call})
			}
			c.Methods = append(c.Methods, bridge)
		}
	}
	for _, c := range l.out.Classes {
		visit(c)
	}
}
