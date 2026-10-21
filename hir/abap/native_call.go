package abap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Match the Go backend's closed native-call catalogue. A prototype-dependent
// callable or a void slot without a value bridge is not a proven JS call.
func (e *emitter) callableClass(c *hir.Class) bool {
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, name := range base.Implements {
			if name == "closure.thunk" || strings.HasPrefix(name, "closure.function.") {
				return true
			}
		}
	}
	return false
}
func (e *emitter) classesByDepth() []*hir.Class {
	out := append([]*hir.Class(nil), e.p.Classes...)
	depth := func(c *hir.Class) int {
		n := 0
		for ; c != nil; c = e.classBy(c.Super) {
			n++
		}
		return n
	}
	sort.SliceStable(out, func(i, j int) bool { return depth(out[i]) > depth(out[j]) })
	return out
}
func (b *body) nativeCall0(x *hir.Expr, n, a string) {
	e := b.e
	if len(x.Args) != 1 || x.Args[0].Kind != hir.Lit {
		e.refuse(fmt.Errorf("node %d (%s): native zero-argument call needs a literal method name", x.ID, x.Source))
		return
	}
	name, ok := x.Args[0].Value.(string)
	if !ok {
		e.refuse(fmt.Errorf("node %d (%s): native call method name", x.ID, x.Source))
		return
	}
	helper := e.name("runtime.native_call0")
	member := e.member(name)
	if e.nativeCalls == nil {
		e.nativeCalls = map[string]string{}
	}
	if _, used := e.nativeCalls[name]; !used {
		shared := &body{e: e, m: &hir.Method{Result: hir.T(hir.Dynamic)}, locals: map[string]string{}}
		shared.nativeCall0Dispatch(name, "result", "a")
		e.nativeCalls[name] = "METHOD " + member + ".\n" + shared.constantDeclarations() + foldTempInits(shared.code.String()) + "ENDMETHOD.\n"
	}
	b.line(n + " = " + helper + "=>" + member + "( " + a + " ).")
}

// nativeCall0Runtime assembles only used dispatchers in source-name order.
func (e *emitter) nativeCall0Runtime() {
	if len(e.nativeCalls) == 0 {
		return
	}
	names := make([]string, 0, len(e.nativeCalls))
	for name := range e.nativeCalls {
		names = append(names, name)
	}
	sort.Strings(names)
	helper, dynamic := e.name("runtime.native_call0"), e.name("runtime.dynamic")
	var def, impl strings.Builder
	for _, name := range names {
		def.WriteString("CLASS-METHODS " + e.member(name) + " IMPORTING a TYPE REF TO " + dynamic + " RETURNING VALUE(result) TYPE REF TO " + dynamic + ".\n")
		impl.WriteString(e.nativeCalls[name])
	}
	e.files[helper+".clas.abap"] = "CLASS " + helper + " DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\n" + def.String() + "PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + helper + " IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
}

// nativeCall0Dispatch preserves the closed catalogue and most-derived-first arms.
func (b *body) nativeCall0Dispatch(name, n, a string) {
	e := b.e
	e.dynamicRuntime()
	trap := e.classRefusal()
	b.line("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
	b.line("IF " + a + "->tag <> " + e.name("runtime.dynamic") + "=>tag_ref OR " + a + "->oval IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
	done := b.temp(hir.T(hir.Bool))
	b.line("CLEAR " + done + ".\nCLEAR " + n + ".")
	for _, c := range e.classesByDepth() {
		unsafe := c.Structural || e.callableClass(c) || e.fieldOwner(c, name) != nil
		for base := c; base != nil; base = e.classBy(base.Super) {
			unsafe = unsafe || base.CoercionUnsafe || base.PrototypeAccessors
		}
		m, _ := e.method(c, name)
		if m == nil {
			continue
		}
		b.line("IF " + done + " = abap_false AND " + a + "->oval IS INSTANCE OF " + e.name(c.Name) + ".")
		if unsafe || m.Static || len(m.Params) != 0 {
			b.line("RAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
			continue
		}
		if m.Result.Kind == hir.Void {
			m, _ = e.method(c, name+"_value")
			if m == nil || m.Static || len(m.Params) != 0 || m.Result.Kind == hir.Void {
				b.line("RAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
				continue
			}
		}
		native := b.temp(hir.Ref(c.Name))
		b.line(native + " ?= " + a + "->oval.")
		value := b.temp(m.Result)
		b.line(value + " = " + native + "->" + e.member(m.Name) + "( ).")
		b.boxDynamic(n, value, m.Result)
		b.line(done + " = abap_true.\nENDIF.")
	}
	b.line("IF " + done + " = abap_false.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
}
