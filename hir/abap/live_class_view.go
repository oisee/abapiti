package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) fieldOwner(c *hir.Class, name string) *hir.Class {
	for ; c != nil; c = e.classBy(c.Super) {
		for _, f := range c.Fields {
			if f.Name == name {
				return c
			}
		}
	}
	return nil
}
func (e *emitter) liveRuntime() {
	if e.types["runtime.liveView"] {
		return
	}
	e.types["runtime.liveView"] = true
	e.classvalueRuntime()
	e.dynamicRuntime()
	d := e.name("runtime.dynamic")
	storage := e.name("view.storage")
	e.files[storage+".intf.abap"] = "INTERFACE " + storage + " PUBLIC.\nMETHODS structural RETURNING VALUE(result) TYPE abap_bool.\nMETHODS source RETURNING VALUE(result) TYPE REF TO " + d + ".\nMETHODS get IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE REF TO " + d + ".\nMETHODS put IMPORTING p0 TYPE string p1 TYPE REF TO " + d + ".\nENDINTERFACE.\n"
	trap := e.name("view.refusal")
	e.files[trap+".clas.abap"] = "CLASS " + trap + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + trap + " IMPLEMENTATION.\nENDCLASS.\n"
	// Native structural shapes expose their existing slots through the same graph API.
	text := e.files[d+".clas.abap"]
	text = strings.Replace(text, "METHOD get.\n", "METHOD get.\nIF tag = tag_ref.\nDATA adapter TYPE REF TO "+storage+".\nTRY.\nadapter ?= oval.\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE "+trap+".\nENDTRY.\nresult = adapter->get( p0 ).\nRETURN.\nENDIF.\n", 1)
	text = strings.Replace(text, "METHOD put.\n", "METHOD put.\nIF tag = tag_ref.\nDATA adapter TYPE REF TO "+storage+".\nTRY.\nadapter ?= oval.\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE "+trap+".\nENDTRY.\nadapter->put( p0 = p0 p1 = p1 ).\nRETURN.\nENDIF.\n", 1)
	e.files[d+".clas.abap"] = text
}
func (e *emitter) liveRoot(c *hir.Class) bool {
	return e.liveClasses[c.Name] && !e.liveClasses[c.Super]
}
func (e *emitter) liveClassDeclaration(b *strings.Builder, c *hir.Class) {
	if len(e.liveClasses) == 0 {
		return
	}
	d := e.name("runtime.dynamic")
	if e.liveRoot(c) {
		fmt.Fprintf(b, "DATA %s TYPE REF TO %s.\nINTERFACES %s.\n", e.name("view.source"), d, e.name("view.storage"))
	}
	if len(e.liveClasses) > 0 && c.Structural && !e.liveClasses[c.Name] {
		fmt.Fprintf(b, "INTERFACES %s.\nDATA %s TYPE REF TO %s.\n", e.name("view.storage"), e.name("view.extras"), d)
	}
	if !e.liveClasses[c.Name] {
		return
	}
	for _, f := range c.Fields {
		if f.Static {
			continue
		}
		fmt.Fprintf(b, "METHODS %s RETURNING VALUE(result) TYPE %s.\nMETHODS %s IMPORTING p0 TYPE %s.\n", e.name("view.read."+c.Name+"."+f.Name), e.typ(f.Type), e.name("view.write."+c.Name+"."+f.Name), e.typ(f.Type))
	}
}
func (e *emitter) liveConstructor(b *body, c *hir.Class) {
	b.line("IF view_source IS BOUND.")
	if c.Super != "" {
		args := []string{"view_source = view_source"}
		if base := e.classBy(c.Super); base != nil && base.Ctor != nil {
			for _, p := range base.Ctor.Params {
				args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
			}
		}
		b.line("super->constructor( " + strings.Join(args, " ") + " ).")
	}
	b.line("me->" + e.name("view.source") + " = view_source.")
	b.line("RETURN.\nENDIF.")
}
func (b *body) liveUnbox(n, d string, t hir.Type) {
	e := b.e
	if t.Kind == hir.Optional {
		b.line("IF " + d + " IS BOUND.")
		if t.Args[0].IsRef() {
			b.liveUnbox(n, d, t.Args[0])
		} else {
			b.line(n + " = NEW #( ).\n" + n + "->has = abap_true.")
			b.liveUnbox(n+"->value", d, t.Args[0])
		}
		b.line("ENDIF.")
		return
	}
	if t.Kind == hir.Dynamic {
		b.line(n + " = " + d + ".")
		return
	}
	switch t.Kind {
	case hir.String:
		b.line(n + " = " + d + "->as_string( ).")
	case hir.Number:
		b.line(n + " = " + d + "->as_number( ).")
	case hir.I32, hir.I64:
		v := b.temp(hir.T(hir.Number))
		b.line(v + " = " + d + "->as_number( ).")
		lo, hi := "-2147483648", "2147483647"
		if t.Kind == hir.I64 {
			lo, hi = "-9007199254740991", "9007199254740991"
		}
		b.line("IF " + v + " <> trunc( " + v + " ) OR " + v + " < '" + lo + "' OR " + v + " > '" + hi + "'.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + ".\nENDIF.")
		b.line(n + " = " + v + ".")
	case hir.Bool:
		b.line(n + " = " + d + "->as_boolean( ).")
	case hir.Array:
		// ABAP typed tables cannot alias a parsed dynamic-element table.
		b.line("IF " + d + " IS NOT BOUND OR " + d + "->tag <> " + e.name("runtime.dynamic") + "=>tag_ref.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + ".\nENDIF.")
		v := b.rawTemp("REF TO object")
		b.line(v + " = " + d + "->as_ref( ).")
		b.line("TRY.\n" + n + " ?= " + v + ".\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + ".\nENDTRY.")
	case hir.ClassRef, hir.InterfaceRef, hir.OrderedMap, hir.OrderedSet:
		v := b.rawTemp("REF TO object")
		b.line(v + " = " + d + "->as_ref( ).")
		b.line(n + " ?= " + v + ".")
	default:
		b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + ".")
	}
}
func (e *emitter) liveClassImplementation(out *strings.Builder, c *hir.Class) {
	if len(e.liveClasses) == 0 {
		return
	}
	makeBody := func(name string) *body {
		return &body{e: e, c: c, m: &hir.Method{Result: hir.T(hir.Void)}, implemented: name, locals: map[string]string{}}
	}
	emit := func(name string, b *body) {
		fmt.Fprintf(out, "METHOD %s.\n%s%sENDMETHOD.\n", name, b.constantDeclarations(), foldTempInits(b.code.String()))
	}
	for _, f := range c.Fields {
		if f.Static || !e.liveClasses[c.Name] {
			continue
		}
		name := e.name("view.read." + c.Name + "." + f.Name)
		b := makeBody(name)
		source := "me->" + e.name("view.source")
		b.line("IF " + source + " IS BOUND.")
		d := b.temp(hir.T(hir.Dynamic))
		b.line(d + " = " + source + "->get( `" + strings.ReplaceAll(f.Name, "`", "``") + "` ).")
		b.liveUnbox("result", d, f.Type)
		b.line("ELSE.\nresult = me->" + e.member(f.Name) + ".\nENDIF.")
		emit(name, b)
		name = e.name("view.write." + c.Name + "." + f.Name)
		b = makeBody(name)
		b.line("IF " + source + " IS BOUND.")
		d = b.temp(hir.T(hir.Dynamic))
		b.boxDynamic(d, "p0", f.Type)
		b.line(source + "->put( p0 = `" + strings.ReplaceAll(f.Name, "`", "``") + "` p1 = " + d + " ).")
		b.line("ELSE.\nme->" + e.member(f.Name) + " = p0.\nENDIF.")
		emit(name, b)
	}
	storage := e.name("view.storage")
	if e.liveRoot(c) {
		fmt.Fprintf(out, "METHOD %s~structural.\nENDMETHOD.\n", storage)
		fmt.Fprintf(out, "METHOD %s~source.\nresult = me->%s.\nENDMETHOD.\nMETHOD %s~get.\nRAISE EXCEPTION TYPE %s.\nENDMETHOD.\nMETHOD %s~put.\nRAISE EXCEPTION TYPE %s.\nENDMETHOD.\n", storage, e.name("view.source"), storage, e.name("view.refusal"), storage, e.name("view.refusal"))
	}
	if len(e.liveClasses) == 0 || !c.Structural || e.liveClasses[c.Name] {
		return
	}
	fmt.Fprintf(out, "METHOD %s~structural.\nresult = abap_true.\nENDMETHOD.\nMETHOD %s~source.\nENDMETHOD.\n", storage, storage)
	b := makeBody(storage + "~get")
	b.line("CASE p0.")
	for _, f := range c.Fields {
		if !f.Static {
			b.line("WHEN `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
			b.boxDynamic("result", "me->"+e.member(f.Name), f.Type)
		}
	}
	b.line("WHEN OTHERS.\nIF me->" + e.name("view.extras") + " IS BOUND.\nresult = me->" + e.name("view.extras") + "->get( p0 ).\nENDIF.\nENDCASE.")
	emit(storage+"~get", b)
	b = makeBody(storage + "~put")
	b.line("CASE p0.")
	for _, f := range c.Fields {
		if !f.Static {
			b.line("WHEN `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
			b.liveUnbox("me->"+e.member(f.Name), "p1", f.Type)
		}
	}
	b.line("WHEN OTHERS.\nIF p0 = `__proto__`.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + ".\nENDIF.\nIF me->" + e.name("view.extras") + " IS NOT BOUND.\nCREATE OBJECT me->" + e.name("view.extras") + ".\nme->" + e.name("view.extras") + "->tag = " + e.name("runtime.dynamic") + "=>tag_object.\nENDIF.\nme->" + e.name("view.extras") + "->put( p0 = p0 p1 = p1 ).\nENDCASE.")
	if e.jsonOwnKeys && c.Structural && c.Super == "" {
		b.jsonRecordKey("me", "p0")
	}
	emit(storage+"~put", b)
}
func (b *body) liveView(x *hir.Expr, n, a string) {
	e := b.e
	d := e.name("runtime.dynamic")
	trap := e.name("view.refusal")
	// Native success remains identity-preserving even for behaviour-bearing classes.
	b.line("IF " + a + " IS BOUND.")
	b.line("IF " + a + "->tag = " + d + "=>tag_ref.\nTRY.\n" + n + " ?= " + a + "->oval.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nENDIF.")
	b.line("IF " + n + " IS NOT BOUND.")
	if !hir.DataClassView(e.p, x.Type.Name) {
		b.line("RAISE EXCEPTION TYPE " + trap + ".")
	} else {
		b.line("IF " + a + "->tag <> " + d + "=>tag_object.")
		adapter := b.rawTemp("REF TO " + e.name("view.storage"))
		b.line("IF " + a + "->tag <> " + d + "=>tag_ref.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
		b.line("TRY.\n" + adapter + " ?= " + a + "->oval.\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE " + trap + ".\nENDTRY.\nIF " + adapter + "->structural( ) <> abap_true.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.\nENDIF.")
		args := []string{"view_source = " + a}
		c := e.classBy(x.Type.Name)
		if c.Ctor != nil {
			for _, p := range c.Ctor.Params {
				args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
			}
		}
		native := e.name(c.Name)
		if c.Abstract {
			native = e.name("view.proxy." + c.Name)
			e.files[native+".clas.abap"] = "CLASS " + native + " DEFINITION PUBLIC INHERITING FROM " + e.name(c.Name) + " CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + native + " IMPLEMENTATION.\nENDCLASS.\n"
		}
		b.line(n + " = NEW " + native + "( " + strings.Join(args, " ") + " ).")
	}
	b.line("ENDIF.\nENDIF.")
}
func (b *body) liveBox(n, a string, t hir.Type) {
	e := b.e
	adapter := b.rawTemp("REF TO " + e.name("view.storage"))
	b.line("TRY.\n" + adapter + " ?= " + a + ".\nCATCH cx_sy_move_cast_error.\nENDTRY.")
	b.line("IF " + adapter + " IS BOUND.\n" + n + " = " + adapter + "->source( ).\nENDIF.")
	b.line("IF " + n + " IS NOT BOUND.\nCREATE OBJECT " + n + ".\n" + n + "->tag = " + e.name("runtime.dynamic") + "=>tag_ref.\n" + n + "->oval = " + a + ".\nENDIF.")
}
