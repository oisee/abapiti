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
	e.files[storage+".intf.abap"] = "INTERFACE " + storage + " PUBLIC.\nMETHODS structural RETURNING VALUE(result) TYPE abap_bool.\nMETHODS source RETURNING VALUE(result) TYPE REF TO " + d + ".\nMETHODS get IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE REF TO " + d + ".\nMETHODS put IMPORTING p0 TYPE string p1 TYPE REF TO " + d + ".\nMETHODS has IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nMETHODS delete IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nENDINTERFACE.\n"
	guard := e.name("view.guard")
	e.files[guard+".intf.abap"] = "INTERFACE " + guard + " PUBLIC.\nMETHODS check IMPORTING p0 TYPE REF TO " + d + ".\nENDINTERFACE.\n"
	trap := e.name("view.refusal")
	e.files[trap+".clas.abap"] = "CLASS " + trap + ` DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.
PUBLIC SECTION.
METHODS constructor IMPORTING reason TYPE string.
METHODS get_text REDEFINITION.
PROTECTED SECTION.
PRIVATE SECTION.
DATA reason TYPE string.
ENDCLASS.
CLASS ` + trap + ` IMPLEMENTATION.
METHOD constructor.
super->constructor( ).
me->reason = reason.
ENDMETHOD.
METHOD get_text.
result = |ABAP view refused: { reason }|.
ENDMETHOD.
ENDCLASS.
`
	// Native structural shapes expose their existing slots through the same graph API.
	text := e.files[d+".clas.abap"]
	text = strings.Replace(text, "METHOD get.\n", "METHOD get.\nIF tag = tag_ref.\nDATA adapter TYPE REF TO "+storage+".\n"+castOrRaise("adapter", "oval", storage, "RAISE EXCEPTION TYPE "+trap+" EXPORTING reason = `live_class_view.go: dynamic.get: reference does not implement view storage`.")+"\nresult = adapter->get( p0 ).\nRETURN.\nENDIF.\n", 1)
	text = strings.Replace(text, "METHOD put.\n", "METHOD put.\nIF tag = tag_ref.\nDATA adapter TYPE REF TO "+storage+".\n"+castOrRaise("adapter", "oval", storage, "RAISE EXCEPTION TYPE "+trap+" EXPORTING reason = `live_class_view.go: dynamic.put: reference does not implement view storage`.")+"\nadapter->put( p0 = p0 p1 = p1 ).\nRETURN.\nENDIF.\n", 1)
	e.files[d+".clas.abap"] = text
	e.recordGraphRuntime()
}
func (e *emitter) liveRoot(c *hir.Class) bool {
	return e.liveClasses[c.Name] && !e.liveClasses[c.Super]
}
func (e *emitter) liveClassDeclaration(b *strings.Builder, c *hir.Class) {
	if !e.types["runtime.liveView"] {
		return
	}
	d := e.name("runtime.dynamic")
	e.ownClassDeclaration(b, c)
	if e.liveRoot(c) {
		fmt.Fprintf(b, "DATA %s TYPE REF TO %s.\n", e.name("view.guardSlot"), e.name("view.guard"))
		fmt.Fprintf(b, "DATA %s TYPE REF TO %s.\nINTERFACES %s.\n", e.name("view.source"), d, e.name("view.storage"))
		fmt.Fprintf(b, "DATA %s TYPE REF TO %s.\nDATA %s TYPE REF TO %s.\n", e.name("view.ownKeys"), d, e.name("view.extras"), d)
	}
	if c.Structural && !e.liveClasses[c.Name] {
		if !e.ownAdapter(c) {
			fmt.Fprintf(b, "INTERFACES %s.\n", e.name("view.storage"))
		}
		fmt.Fprintf(b, "DATA %s TYPE REF TO %s.\n", e.name("view.extras"), d)
		fmt.Fprintf(b, "DATA %s TYPE REF TO %s.\n", e.name("view.ownKeys"), d)
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

func (e *emitter) constructorExprs(s *hir.Stmt, visit func(*hir.Expr)) {
	var walk func(*hir.Expr)
	walk = func(x *hir.Expr) {
		if x == nil {
			return
		}
		visit(x)
		walk(x.X)
		walk(x.Y)
		walk(x.Z)
		for _, arg := range x.Args {
			walk(arg)
		}
		e.walkStmt(x.Stmt, walk)
	}
	e.walkStmt(s, walk)
}

// splitSuperConstructor retains native parameter defaults and argument setup
// before a unique top-level call. Conditional calls, early returns and access
// to THIS before SUPER cannot be moved safely and remain explicit refusals.
func (e *emitter) splitSuperConstructor(body *hir.Stmt) (super *hir.Expr, before, after *hir.Stmt, valid bool) {
	calls := 0
	e.constructorExprs(body, func(x *hir.Expr) {
		if x != nil && x.Kind == hir.SuperCall && x.Name == "constructor" {
			calls++
		}
	})
	if calls == 0 {
		return nil, nil, body, true
	}
	if calls != 1 {
		return nil, nil, body, false
	}
	parts := []*hir.Stmt{body}
	if body != nil && body.Kind == hir.Block {
		parts = body.List
	}
	for i, part := range parts {
		if part == nil || part.Kind != hir.ExprStmt || part.X == nil || part.X.Kind != hir.SuperCall || part.X.Name != "constructor" {
			continue
		}
		before, after = hir.B(parts[:i]...), hir.B(parts[i+1:]...)
		unsafe := false
		var returns func(*hir.Stmt)
		returns = func(s *hir.Stmt) {
			if s == nil {
				return
			}
			if s.Kind == hir.Return {
				unsafe = true
			}
			returns(s.Body)
			returns(s.Else)
			for _, child := range s.List {
				returns(child)
			}
		}
		returns(before)
		e.constructorExprs(hir.B(before, part), func(x *hir.Expr) {
			if x != nil && x.Kind == hir.This {
				unsafe = true
			}
			if x != nil {
				returns(x.Stmt)
			}
		})
		return part.X, before, after, !unsafe
	}
	return nil, nil, body, false
}

// liveConstructor joins the native and view paths before invoking SUPER. A view
// supplies initial argument slots without evaluating native argument expressions;
// every ancestor receives the same source and skips its native initializer.
func (e *emitter) liveConstructor(b *body, c *hir.Class, m *hir.Method) *hir.Stmt {
	body := m.Body
	// Lazy static initialization used to precede native argument evaluation.
	// A structural view must continue to skip it.
	if e.staticInit[c.Name] {
		b.line("IF view_source IS NOT BOUND.")
		b.initialize(c.Name)
		b.line("ENDIF.")
	}
	if c.Super != "" {
		super, before, after, valid := e.splitSuperConstructor(body)
		if !valid {
			e.refuse(fmt.Errorf("class %s constructor: requires a single top-level super constructor call without early return or THIS access", c.Name))
			return body
		}
		parent := e.p.Constructor(c.Super)
		if super != nil && (parent == nil && len(super.Args) > 0 || parent != nil && len(super.Args) > len(parent.Params)) {
			e.refuse(fmt.Errorf("class %s constructor: superclass argument signature is unknown", c.Name))
			return body
		}
		args := []string{}
		if super != nil {
			// Evaluation may have side effects or throw: it belongs only to the
			// native path. The call itself must remain outside this control block.
			b.line("IF view_source IS NOT BOUND.")
			for _, p := range m.Params {
				n := b.temp(p.Type)
				b.locals[p.Name] = n
				b.line(n + " = " + e.param(p.Name) + ".")
			}
			b.stmt(before)
			for i, a := range super.Args {
				p := parent.Params[i]
				n := b.temp(p.Type)
				b.line(n + " = " + b.value(a, p.Type) + ".")
				args = append(args, e.param(p.Name)+" = "+n)
			}
			b.line("ENDIF.")
			body = after
		} else if !e.needsImplicitSuper(c, m) {
			e.refuse(fmt.Errorf("class %s constructor: missing required superclass arguments", c.Name))
		}
		if e.liveClasses[c.Super] {
			args = append(args, "view_source = view_source")
		}
		b.line("super->constructor( " + strings.Join(args, " ") + " ).")
	}
	b.line("IF view_source IS BOUND.")
	b.line("me->" + e.name("view.source") + " = view_source.")
	b.line("RETURN.\nENDIF.")
	return body
}
func (b *body) liveUnbox(n, d string, t hir.Type) {
	e := b.e
	if t.Kind == hir.Optional {
		b.line("CLEAR " + n + ".")
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
	if t.Kind == hir.UnionRef {
		b.line(n + " = " + e.unionFamily(t.Name, "") + "=>check( " + d + " ).")
		return
	}
	switch t.Kind {
	case hir.String:
		b.line(n + " = " + d + "->as_string( ).")
	case hir.Number:
		b.line("IF " + d + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: liveUnbox: undefined cannot inhabit required slot`.\nENDIF.")
		b.line(n + " = " + d + "->as_number( ).")
	case hir.I32, hir.I64:
		v := b.temp(hir.T(hir.Number))
		b.line(v + " = " + d + "->as_number( ).")
		lo, hi := "-2147483648", "2147483647"
		if t.Kind == hir.I64 {
			lo, hi = "-9007199254740991", "9007199254740991"
		}
		b.line("IF " + v + " <> trunc( " + v + " ) OR " + v + " < '" + lo + "' OR " + v + " > '" + hi + "'.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: liveUnbox: integer is fractional or outside exact range`.\nENDIF.")
		b.line(n + " = " + v + ".")
	case hir.Bool:
		// dynBoolean refuses an absent value in Go with a range fault; a view
		// read of a missing property yields an unbound box, and the typed
		// boundary must refuse it instead of dereferencing it.
		b.line("IF " + d + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: liveUnbox: undefined cannot inhabit required slot`.\nENDIF.")
		b.line(n + " = " + d + "->as_boolean( ).")
	case hir.ClassValue:
		// Erased reference arrays can carry boxed constructor descriptors.
		// Preserve the descriptor object; as_classvalue checks the tag.
		b.line(n + " = " + d + "->as_classvalue( ).")
	case hir.Array:
		b.arrayView(t, n, d)
	case hir.ClassRef:
		// Parsed child records retain the same backing graph as their parent.
		// Use the same activated-layout boundary as Go. Other nominal fields
		// still require compatible references; no prototype is fabricated.
		if e.liveClasses[t.Name] {
			b.liveView(&hir.Expr{Type: t}, n, d)
			return
		}
		fallthrough
	case hir.InterfaceRef:
		if view := e.structuralUnion(t); view != "" {
			b.line(n + " = " + view + "=>decode( " + d + " ).")
			return
		}
		v := b.rawTemp("REF TO object")
		b.line(v + " = " + d + "->as_ref( ).")
		b.line(n + " ?= " + v + ".")
	case hir.OrderedMap, hir.OrderedSet:
		v := b.rawTemp("REF TO object")
		b.line(v + " = " + d + "->as_ref( ).")
		b.line(n + " ?= " + v + ".")
	default:
		b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: liveUnbox: slot has no exact unbox ABI`.")
	}
}
func (e *emitter) liveClassImplementation(out *strings.Builder, c *hir.Class) {
	if !e.types["runtime.liveView"] {
		return
	}
	if e.refuseInheritedOwnAdapter(out, c) {
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
		guard := "me->" + e.name("view.guardSlot")
		b.line("IF " + guard + " IS BOUND.\n" + guard + "->check( " + source + " ).\nENDIF.")
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
		b.line("ELSE.\nme->" + e.member(f.Name) + " = p0.")
		if c.Structural {
			b.markOwnKey("me", f.Name)
		}
		if e.recordDelete {
			b.line("DELETE me->" + e.name("record.deleted") + " WHERE table_line = `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
		}
		b.line("ENDIF.")
		emit(name, b)
	}
	storage := e.name("view.storage")
	if e.liveRoot(c) && !c.Structural && !e.ownAdapter(c) {
		fallback := "RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: nominal.delete: no backing source or deletion adapter`."
		if e.recordDelete {
			fallback = "result = " + e.nativeRecordDeleteRuntime() + "=>run( p0 = me key = p0 ).\nRETURN."
		}
		fmt.Fprintf(out, "METHOD %s~has.\nIF me->%s IS NOT BOUND.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `live_class_view.go: nominal.has: no backing source`.\nENDIF.\nresult = me->%s->has( p0 ).\nENDMETHOD.\nMETHOD %s~delete.\nIF me->%s IS NOT BOUND.\n%s\nENDIF.\nresult = me->%s->delete( p0 ).\nENDMETHOD.\n", storage, e.name("view.source"), e.name("view.refusal"), e.name("view.source"), storage, e.name("view.source"), fallback, e.name("view.source"))
		fmt.Fprintf(out, "METHOD %s~structural.\nENDMETHOD.\n", storage)
		fmt.Fprintf(out, "METHOD %s~source.\nresult = me->%s.\nENDMETHOD.\nMETHOD %s~get.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `live_class_view.go: nominal.get: generic native slot reads unavailable`.\nENDMETHOD.\nMETHOD %s~put.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `live_class_view.go: nominal.put: generic native slot writes unavailable`.\nENDMETHOD.\n", storage, e.name("view.source"), storage, e.name("view.refusal"), storage, e.name("view.refusal"))
	}
	if e.ownAdapter(c) {
		e.ownClassImplementation(out, c)
		return
	}
	if !e.types["runtime.liveView"] || !c.Structural || e.liveClasses[c.Name] && !e.liveRoot(c) {
		return
	}
	fmt.Fprintf(out, "METHOD %s~structural.\nresult = abap_true.\nENDMETHOD.\nMETHOD %s~source.\n", storage, storage)
	if e.liveRoot(c) {
		fmt.Fprintf(out, "result = me->%s.\n", e.name("view.source"))
	}
	out.WriteString("ENDMETHOD.\n")
	e.nativeStructuralPresence(out, c)
	b := makeBody(storage + "~get")
	if e.liveRoot(c) {
		b.line("IF me->" + e.name("view.source") + " IS BOUND.\nresult = me->" + e.name("view.source") + "->get( p0 ).\nRETURN.\nENDIF.")
	}
	b.line("IF me->" + e.name("view.ownKeys") + " IS BOUND AND me->" + e.name("view.ownKeys") + "->has( p0 ) = abap_false.\nIF me->" + e.name("view.extras") + " IS NOT BOUND.\nRETURN.\nENDIF.\nIF me->" + e.name("view.extras") + "->has( p0 ) = abap_false.\nRETURN.\nENDIF.\nENDIF.")
	b.line("CASE p0.")
	for _, f := range c.Fields {
		if !f.Static {
			b.line("WHEN `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
			if f.Type.Kind == hir.I64 || f.Type.Kind == hir.Optional && f.Type.Args[0].Kind == hir.I64 {
				b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: structural.get: i64 boxing unavailable`.")
			} else {
				b.boxDynamic("result", "me->"+e.member(f.Name), f.Type)
			}
		}
	}
	b.line("WHEN OTHERS.")
	e.refuseUnknownNativeSubclass(b, c)
	b.line("IF me->" + e.name("view.extras") + " IS BOUND.\nresult = me->" + e.name("view.extras") + "->get( p0 ).\nENDIF.\nENDCASE.")
	emit(storage+"~get", b)
	b = makeBody(storage + "~put")
	if e.liveRoot(c) {
		b.line("IF me->" + e.name("view.source") + " IS BOUND.\nme->" + e.name("view.source") + "->put( p0 = p0 p1 = p1 ).\nRETURN.\nENDIF.")
	}
	b.line("CASE p0.")
	for _, f := range c.Fields {
		if !f.Static {
			b.line("WHEN `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
			value := b.temp(f.Type)
			b.line("CLEAR " + value + ".")
			b.liveUnbox(value, "p1", f.Type)
			b.line("me->" + e.member(f.Name) + " = " + value + ".")
			b.markOwnKey("me", f.Name)
			if e.recordDelete {
				b.line("DELETE me->" + e.name("record.deleted") + " WHERE table_line = `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
			}
		}
	}
	b.line("WHEN OTHERS.")
	e.refuseUnknownNativeSubclass(b, c)
	b.line("IF p0 = `__proto__`.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: structural.put: prototype mutation unavailable`.\nENDIF.\nIF me->" + e.name("view.extras") + " IS NOT BOUND.\nCREATE OBJECT me->" + e.name("view.extras") + ".\nme->" + e.name("view.extras") + "->tag = " + e.name("runtime.dynamic") + "=>tag_object.\nENDIF.\nme->" + e.name("view.extras") + "->put( p0 = p0 p1 = p1 ).\nENDCASE.")
	if e.jsonOwnKeys && c.Structural && c.Super == "" {
		b.jsonRecordKey("me", "p0")
	}
	emit(storage+"~put", b)
}
func (b *body) liveView(x *hir.Expr, n, a string, fresh ...bool) {
	e := b.e
	d := e.name("runtime.dynamic")
	trap := e.name("view.refusal")
	b.line("CLEAR " + n + ".")
	// Native success remains identity-preserving even for behaviour-bearing classes.
	b.line("IF " + a + " IS BOUND AND " + a + "->tag <> " + d + "=>tag_null.")
	if len(fresh) == 0 || !fresh[0] {
		b.line("IF " + a + "->tag = " + d + "=>tag_ref.\n" + clearedCastProbe(n, a+"->oval", e.name(x.Type.Name)) + "\nENDIF.")
	}
	b.line("IF " + n + " IS NOT BOUND.")
	if !hir.DataClassView(e.p, x.Type.Name) {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_class_view.go: liveView: behaviour-bearing class requires compatible native reference`.")
	} else {
		allowed := a + "->tag <> " + d + "=>tag_object"
		for c := e.classBy(x.Type.Name); c != nil; c = e.classBy(c.Super) {
			if c.PrimitiveReads {
				for _, tag := range []string{"tag_string", "tag_number", "tag_boolean"} {
					allowed += " AND " + a + "->tag <> " + d + "=>" + tag
				}
				break
			}
		}
		b.line("IF " + allowed + ".")
		adapter := b.rawTemp("REF TO " + e.name("view.storage"))
		b.line("IF " + a + "->tag <> " + d + "=>tag_ref.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_class_view.go: liveView: source is neither object graph nor reference`.\nENDIF.")
		b.line(castOrRaise(adapter, a+"->oval", e.name("view.storage"), "RAISE EXCEPTION TYPE "+trap+" EXPORTING reason = `live_class_view.go: liveView: reference does not implement view storage`.") + "\nIF " + adapter + "->structural( ) <> abap_true.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_class_view.go: liveView: reference storage is not structural`.\nENDIF.\nENDIF.")
		c := e.classBy(x.Type.Name)
		native := e.name(c.Name)
		if c.Abstract {
			native = e.name("view.proxy." + c.Name)
			e.files[native+".clas.abap"] = "CLASS " + native + " DEFINITION PUBLIC INHERITING FROM " + e.name(c.Name) + " CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + native + " IMPLEMENTATION.\nENDCLASS.\n"
		}
		if e.wideShape(c) {
			// A wide shape has no constructor: bind the source as its constructor would.
			b.line(n + " = NEW " + native + "( ).\n" + n + "->" + e.name("view.source") + " = " + a + ".")
		} else {
			args := []string{"view_source = " + a}
			if c.Ctor != nil {
				for _, p := range c.Ctor.Params {
					args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
				}
			}
			b.line(n + " = NEW " + native + "( " + strings.Join(args, " ") + " ).")
		}
	}
	b.line("ENDIF.\nENDIF.")
}
func referenceStorage(t hir.Type) bool {
	return t.IsRef() || t.Kind == hir.Optional && t.Args[0].IsRef()
}

func (b *body) liveBox(n, a string, t hir.Type) {
	e := b.e
	adapter := b.rawTemp("REF TO " + e.name("view.storage"))
	raw := b.rawTemp("REF TO object")
	b.line("CLEAR " + adapter + ".")
	b.line(raw + " = " + a + ".")
	b.line(castProbe(n, raw, e.name("runtime.dynamic")) + "\nIF " + n + " IS NOT BOUND.")
	b.line(clearedCastProbe(adapter, raw, e.name("view.storage")))
	b.line("IF " + adapter + " IS BOUND.\n" + n + " = " + adapter + "->source( ).\nENDIF.")
	b.line("IF " + n + " IS NOT BOUND.\nCREATE OBJECT " + n + ".\n" + n + "->tag = " + e.name("runtime.dynamic") + "=>tag_ref.\n" + n + "->oval = " + a + ".\nENDIF.\nENDIF.")
}

// An inherited adapter lacks the child's native slot map. Original graph
// sources delegate before this guard; source-less unknown slots refuse.
func (e *emitter) refuseUnknownNativeSubclass(b *body, c *hir.Class) {
	for _, child := range e.descendants(c) {
		b.line("IF me IS INSTANCE OF " + e.name(child.Name) + ".\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_class_view.go: nativeSubclass: inherited adapter lacks child slot layout`.\nENDIF.")
	}
}
