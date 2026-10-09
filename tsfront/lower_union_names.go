package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// ABAP interface forwarders call the class method with the interface's
// parameter names, so every slot a class implements must agree on them. A
// synthesized union view therefore takes the names of the slot its
// constituents already implement: a declared constituent interface, a declared
// interface of a constituent class, or the root virtual declaration in its
// class chain. Constituents whose own names differ are bridged by
// eraseGenericOverrides like any other slot mismatch.
func (l *lowerer) unionSlotParams(parts []hir.Type, name string) []hir.Param {
	for _, part := range parts {
		if part.Kind == hir.InterfaceRef && !strings.HasPrefix(part.Name, "union.") {
			if m := l.interfaceMethod(part.Name, name); m != nil {
				return m.Params
			}
		}
	}
	for _, part := range parts {
		if part.Kind != hir.ClassRef {
			continue
		}
		for c := l.classByName(part.Name); c != nil; c = l.classByName(c.Super) {
			for _, iface := range c.Implements {
				if strings.HasPrefix(iface, "union.") {
					continue
				}
				if m := l.interfaceMethod(iface, name); m != nil {
					return m.Params
				}
			}
		}
	}
	for _, part := range parts {
		if part.Kind != hir.ClassRef {
			continue
		}
		var root *hir.Method
		for c := l.classByName(part.Name); c != nil; c = l.classByName(c.Super) {
			for _, m := range c.Methods {
				if m.Name == name && m.Virtual && !m.Static {
					root = m
				}
			}
		}
		if root != nil {
			return root.Params
		}
	}
	return nil
}

func (l *lowerer) interfaceMethod(iface, name string) *hir.Method {
	for _, i := range l.out.Interfaces {
		if i.Name != iface {
			continue
		}
		for _, m := range i.Methods {
			if m.Name == name {
				return m
			}
		}
		return nil
	}
	return nil
}

// renameUnionParams applies the slot's parameter names to a view method.
func (l *lowerer) renameUnionParams(parts []hir.Type, name string, params []hir.Param) {
	slot := l.unionSlotParams(parts, name)
	for j := range params {
		if j < len(slot) {
			params[j].Name = slot[j].Name
		}
	}
}
