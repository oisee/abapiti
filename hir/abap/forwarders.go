package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Erased slots and checker-typed implementations.
//
// A class implementing an erased slot (an interface method taking `any`, a
// base method taking a subclass) with a differently typed signature has two
// ABAP methods: the slot and the specialized implementation. The body lives
// in the specialized method, whose parameters are its true types; the slot
// forwards after converting each parameter (unboxing a tagged value, casting
// a reference). Only a body that calls SUPER->slot must sit in the slot
// itself (ABAP allows SUPER->m in METHOD m alone); the specialized method
// then forwards into the slot instead (narrowedBridge).

// forwardsToImplementation reports whether the slot can forward to the
// specialized implementation.
func (e *emitter) forwardsToImplementation(impl, slot *hir.Method) bool {
	return impl.Body != nil && !hasSuperCallTo(impl.Body, slot.Name)
}

func hasSuperCallTo(s *hir.Stmt, name string) bool {
	if s == nil {
		return false
	}
	if exprHasSuperCallTo(s.X, name) || exprHasSuperCallTo(s.Y, name) || hasSuperCallTo(s.Body, name) || hasSuperCallTo(s.Else, name) {
		return true
	}
	for _, x := range s.List {
		if hasSuperCallTo(x, name) {
			return true
		}
	}
	return false
}

func exprHasSuperCallTo(x *hir.Expr, name string) bool {
	if x == nil {
		return false
	}
	if x.Kind == hir.SuperCall {
		original, _, _ := strings.Cut(x.Name, "_instantiated_")
		if original == name {
			return true
		}
	}
	if exprHasSuperCallTo(x.X, name) || exprHasSuperCallTo(x.Y, name) || exprHasSuperCallTo(x.Z, name) || hasSuperCallTo(x.Stmt, name) {
		return true
	}
	for _, a := range x.Args {
		if exprHasSuperCallTo(a, name) {
			return true
		}
	}
	return false
}

// slotForwarder emits the erased slot: convert each parameter to the
// implementation's type, call it, convert the result back.
func (e *emitter) slotForwarder(c *hir.Class, slot, impl *hir.Method) string {
	emitted := e.emittedMethod(c, slot)
	b := &body{e: e, c: c, m: emitted, implemented: e.member(slot.Name), locals: map[string]string{}}
	args := []string{}
	for j, p := range impl.Params {
		if j >= len(slot.Params) {
			e.err = fmt.Errorf("class %s method %s: implementation declares more parameters than its slot", c.Name, slot.Name)
			return ""
		}
		inherited := slot.Params[j]
		if inherited.Type.Kind == hir.Optional && !inherited.Type.Args[0].IsRef() && p.Type.Kind != hir.Optional {
			e.err = fmt.Errorf("class %s method %s parameter %s: narrowing an inherited optional primitive cannot preserve undefined", c.Name, slot.Name, p.Name)
			return ""
		}
		args = append(args, e.param(p.Name)+" = "+b.convert(e.param(inherited.Name), inherited.Type, p.Type))
	}
	call := "me->" + e.member(impl.Name) + "( " + strings.Join(args, " ") + " )"
	switch {
	case emitted.Result.Kind == hir.Void:
		b.line(call + ".")
	case impl.Result.Kind == hir.Void:
		e.err = fmt.Errorf("class %s method %s: void implementation of a value-returning slot", c.Name, slot.Name)
		return ""
	default:
		result := b.temp(impl.Result)
		b.line(result + " = " + call + ".")
		b.line("result = " + b.convert(result, impl.Result, emitted.Result) + ".")
	}
	b.line("RETURN.")
	return "METHOD " + b.implemented + ".\n" + b.code.String() + "ENDMETHOD.\n"
}

// upcast reports whether a reference of type src is assignable to dst without
// a checked cast: the same type, a superclass, or an implemented interface.
func (e *emitter) upcast(src, dst hir.Type) bool {
	if src.Equal(dst) {
		return true
	}
	if dst.Kind == hir.ClassRef && dst.Name == hir.RootObject {
		return true
	}
	if src.Kind != hir.ClassRef {
		return false
	}
	for c := e.classBy(src.Name); c != nil; c = e.classBy(c.Super) {
		if dst.Kind == hir.ClassRef && c.Name == dst.Name {
			return true
		}
		for _, i := range c.Implements {
			if dst.Kind == hir.InterfaceRef && i == dst.Name {
				return true
			}
		}
	}
	return false
}

// unbox converts a tagged value to the primitive or reference dst.
func (b *body) unbox(value string, dst hir.Type) string {
	n := b.temp(dst)
	switch dst.Kind {
	case hir.String:
		b.line(n + " = " + value + "->as_string( ).")
	case hir.Number:
		b.line(n + " = " + value + "->as_number( ).")
	case hir.Bool:
		b.line(n + " = " + value + "->as_boolean( ).")
	case hir.ClassValue:
		b.line(n + " = " + value + "->as_classvalue( ).")
	default:
		root := b.rawTemp("REF TO object")
		b.line(root + " = " + value + "->as_ref( ).")
		b.line(n + " ?= " + root + ".")
	}
	return n
}
