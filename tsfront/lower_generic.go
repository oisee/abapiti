package tsfront

import (
	"github.com/oisee/abapiti/hir"
)

// eraseGenericOverrides fixes inherited and interface ABIs before bodies are
// lowered, including nongeneric covariant returns. The virtual slot retains
// the declaration signature; a separate method records the checker signature.
// The ABAP emitter places inherited bodies in their redefinition slots.
func (l *lowerer) eraseGenericOverrides() {
	if l.bridgeTargets == nil {
		l.bridgeTargets = map[*hir.Method]*hir.Method{}
	}
	classes := map[string]*hir.Class{}
	for _, c := range l.classes {
		classes[c.Name] = c
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
		if base != nil {
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
			if m.Result.Kind == hir.Optional && m.Result.Args[0].Kind == hir.Dynamic && slot.Result.Kind == hir.Optional {
				// `getSuperClass(): undefined` implements an optional slot.
				m.Result = slot.Result
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
			l.bridgeTargets[bridge] = m
			args := []*hir.Expr{}
			for j, p := range slot.Params {
				bridge.Params = append(bridge.Params, p)
				if j >= len(m.Params) {
					continue
				}
				actual := m.Params[j]
				x := hir.V(p.Name, p.Type)
				if actual.Type.Kind == hir.Optional && actual.Type.Args[0].Equal(p.Type) {
					// The implementation accepts absence too: widen, never check.
					x = &hir.Expr{Kind: hir.Conditional, Type: actual.Type, X: hir.L(hir.T(hir.Bool), true), Y: x, Z: &hir.Expr{Kind: hir.Lit, Type: actual.Type}}
				} else if !p.Type.Equal(actual.Type) && !(actual.Type.Kind == hir.ClassRef && actual.Type.Name == hir.RootObject) {
					x = &hir.Expr{Kind: hir.Narrow, Type: actual.Type, X: x}
				}
				args = append(args, x)
			}
			call := &hir.Expr{Kind: hir.VirtualCall, Name: m.Name, Type: m.Result, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}, Args: args}
			if slot.Result.Kind == hir.Void && m.Result.IsRef() {
				// TypeScript permits a value-returning implementation of a
				// void interface method. Keep a stable virtual value slot too,
				// for fluent calls whose result is used by class receivers.
				value := &hir.Method{Node: m.Node, Name: name + "_value", Virtual: true, Result: hir.Ref(hir.RootObject), Params: append([]hir.Param(nil), bridge.Params...), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: call})}
				c.Methods = append(c.Methods, value)
				l.bridgeTargets[value] = m
			}
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

// Excluded implementations must raise their located coverage trap through
// every erased entry slot. Their parameter casts cannot be reached in JS and
// must not turn a coverage trap into a different cast failure in ABAP.
func (l *lowerer) propagateBridgeTraps() {
	visited := map[*hir.Method]bool{}
	var visit func(*hir.Method)
	visit = func(bridge *hir.Method) {
		if visited[bridge] {
			return
		}
		visited[bridge] = true
		target := l.bridgeTargets[bridge]
		if target == nil {
			return
		}
		visit(target)
		if target.Body != nil && target.Body.Kind == hir.Block && len(target.Body.List) == 1 && target.Body.List[0].Kind == hir.Trap {
			trap := *target.Body.List[0]
			bridge.Body = hir.B(&trap)
		}
	}
	for bridge := range l.bridgeTargets {
		visit(bridge)
	}
}
