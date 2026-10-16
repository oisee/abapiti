package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Typed tables are temporary read buffers when source is bound. Every read
// refreshes from the original graph; writes publish to that same graph node.
// No buffer escapes as the identity of the parsed array.
func liveArrayMethod(name, source string) string {
	prefix := "load( ).\n"
	switch name {
	case "concat":
		prefix += "p0->load( ).\n"
	}
	switch name {
	case "push", "unshift", "reverse", "pop", "shift", "splice1", "splice1_view", "splice2", "splice3":
		source = strings.ReplaceAll(source, "RETURN.", "save( ).\nRETURN.") + "save( ).\n"
	}
	return prefix + source
}
func (e *emitter) liveArrayRuntime(t hir.Type) {
	name := e.name("runtime." + t.String())
	file := name + ".clas.abap"
	d, storage, trap := e.name("runtime.dynamic"), e.name("view.storage"), e.name("view.refusal")
	def := "DATA source TYPE REF TO " + d + ".\nMETHODS load.\nMETHODS save.\nINTERFACES " + storage + ".\n"
	code := new(strings.Builder)
	makeBody := func() *body { return &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}} }
	b := makeBody()
	b.line("IF source IS NOT BOUND.\nRETURN.\nENDIF.\nCLEAR items.\nmissing = source->missing.")
	if t.Args[0].Kind == hir.Dynamic || t.Args[0].Name == hir.RootObject {
		// An erased view can also read a native scalar/reference array through
		// its storage adapter. Do not downcast incompatible table classes.
		adapter := b.rawTemp("REF TO " + storage)
		length := b.temp(hir.T(hir.Dynamic))
		index := b.temp(hir.T(hir.I32))
		key := b.temp(hir.T(hir.String))
		item := b.temp(hir.T(hir.Dynamic))
		b.line("IF source->tag = " + d + "=>tag_ref.\n" + adapter + " ?= source->oval.\n" + length + " = " + adapter + "->get( `length` ).\n" + index + " = 0.\nWHILE " + index + " < " + length + "->nval.\n" + key + " = |{ " + index + " }|.\n" + item + " = " + adapter + "->get( " + key + " ).\nAPPEND " + item + " TO items.\n" + index + " = " + index + " + 1.\nENDWHILE.\nRETURN.\nENDIF.")
	}
	idx := b.temp(hir.T(hir.I32))
	node := b.temp(hir.T(hir.Dynamic))
	v := b.temp(t.Args[0])
	b.line("LOOP AT source->items INTO " + node + ".\n" + idx + " = sy-tabix.\nCLEAR " + v + ".\nREAD TABLE missing WITH TABLE KEY table_line = " + idx + " TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.")
	b.line("IF " + node + " IS BOUND.")
	if t.Args[0].Name == hir.RootObject {
		b.line("IF " + node + "->tag = " + d + "=>tag_ref.\n" + v + " = " + node + "->as_ref( ).\nELSE.\n" + v + " = " + node + ".\nENDIF.")
	} else {
		b.liveUnbox(v, node, t.Args[0])
	}
	b.line("ELSE.")
	if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: load: undefined cannot inhabit required element`.")
	}
	b.line("ENDIF.\nENDIF.\nAPPEND " + v + " TO items.\nENDLOOP.")
	code.WriteString("METHOD load.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n")
	b = makeBody()
	b.line("IF source IS NOT BOUND.\nRETURN.\nENDIF.")
	wideInteger := t.Args[0].Kind == hir.I64 || t.Args[0].Kind == hir.Optional && t.Args[0].Args[0].Kind == hir.I64
	if wideInteger {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: save: i64 graph publication unavailable`.")
	}
	b.line("CLEAR source->items.\nsource->missing = missing.")
	v = b.temp(t.Args[0])
	node = b.temp(hir.T(hir.Dynamic))
	b.line("LOOP AT items INTO " + v + ".\nCLEAR " + node + ".")
	if t.Args[0].Kind == hir.I64 || t.Args[0].Kind == hir.Optional && t.Args[0].Args[0].Kind == hir.I64 {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: save: i64 element boxing unavailable`.")
	} else {
		b.boxDynamic(node, v, t.Args[0])
	}
	b.line("APPEND " + node + " TO source->items.\nENDLOOP.")
	code.WriteString("METHOD save.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n")
	code.WriteString("METHOD " + storage + "~source.\nresult = source.\nENDMETHOD.\nMETHOD " + storage + "~structural.\nENDMETHOD.\nMETHOD " + storage + "~has.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: has: live-array key presence unavailable`.\nENDMETHOD.\nMETHOD " + storage + "~delete.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: delete: live-array key deletion unavailable`.\nENDMETHOD.\n")
	b = makeBody()
	idx = b.temp(hir.T(hir.I32))
	node = b.temp(hir.T(hir.Dynamic))
	v = b.temp(t.Args[0])
	b.line("load( ).\nIF p0 = `length`.\nCREATE OBJECT result.\nresult->tag = " + d + "=>tag_number.\nresult->nval = lines( items ).\nRETURN.\nENDIF.\nCREATE OBJECT " + node + ".\n" + idx + " = " + node + "->array_index( p0 ).\nIF has( " + idx + " ) = abap_false.\nRETURN.\nENDIF.\nREAD TABLE items INDEX " + idx + " + 1 INTO " + v + ".")
	if wideInteger {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: get: i64 element boxing unavailable`.")
	} else {
		b.boxDynamic("result", v, t.Args[0])
	}
	code.WriteString("METHOD " + storage + "~get.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n")
	b = makeBody()
	idx = b.temp(hir.T(hir.I32))
	node = b.temp(hir.T(hir.Dynamic))
	v = b.temp(t.Args[0])
	b.line("CREATE OBJECT " + node + ".\n" + idx + " = " + node + "->array_index( p0 ).")
	// Preserve undefined/reference identity; required scalar slots refuse undefined.
	b.line("IF p1 IS BOUND.")
	if t.Args[0].Name == hir.RootObject {
		b.line("IF p1->tag = " + d + "=>tag_ref.\n" + v + " = p1->as_ref( ).\nELSE.\n" + v + " = p1.\nENDIF.")
	} else {
		b.liveUnbox(v, "p1", t.Args[0])
	}
	b.line("ELSE.")
	if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: put: undefined cannot inhabit required element`.")
	}
	b.line("ENDIF.\nput( p0 = " + idx + " p1 = " + v + " ).")
	code.WriteString("METHOD " + storage + "~put.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n")
	// Reference views store graph nodes; normalize both operands to their
	// original identities before searching, rather than compare adapter objects.
	if t.Args[0].Name == hir.RootObject {
		b = makeBody()
		row := b.temp(t.Args[0])
		left := b.temp(hir.T(hir.Dynamic))
		right := b.temp(hir.T(hir.Dynamic))
		index := b.temp(hir.T(hir.I32))
		b.line("dense( ).\nresult = -1.")
		b.boxDynamic(right, "p0", t.Args[0])
		b.line("LOOP AT items INTO " + row + ".\n" + index + " = " + index + " + 1.\nCLEAR " + left + ".")
		b.boxDynamic(left, row, t.Args[0])
		b.line("IF " + left + " IS NOT BOUND.\nIF " + right + " IS NOT BOUND.\nresult = " + index + " - 1.\nRETURN.\nENDIF.\nELSEIF " + left + "->strict_equals( " + right + " ) = abap_true.\nresult = " + index + " - 1.\nRETURN.\nENDIF.\nENDLOOP.")
		text := e.files[file]
		start := strings.Index(text, "METHOD indexOf.")
		end := strings.Index(text[start:], "ENDMETHOD.") + start + len("ENDMETHOD.")
		text = text[:start] + "METHOD indexOf.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD." + text[end:]
		start = strings.Index(text, "METHOD includes.")
		end = strings.Index(text[start:], "ENDMETHOD.") + start + len("ENDMETHOD.")
		e.files[file] = text[:start] + "METHOD includes.\nresult = xsdbool( indexOf( p0 ) >= 0 ).\nENDMETHOD." + text[end:]
	}
	// Existing method bodies are wrapped after sparse helpers have been built.
	text := e.files[file]
	text = strings.Replace(text, "PUBLIC SECTION.\n", "PUBLIC SECTION.\n"+def, 1)
	e.files[file] = strings.TrimSuffix(text, "ENDCLASS.\n") + code.String() + "ENDCLASS.\n"
}
func (b *body) arrayView(t hir.Type, n, a string) {
	e := b.e
	e.liveRuntime()
	trap, d := e.name("view.refusal"), e.name("runtime.dynamic")
	if t.Args[0].Kind == hir.Dynamic {
		e.isArrayUsed = true
		b.line("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `" + "arrayView: undefined is not array storage" + "`.\nENDIF.")
		b.line("IF " + a + "->tag <> " + d + "=>tag_array.\nIF " + a + "->tag <> " + d + "=>tag_ref OR " + a + "->oval IS NOT INSTANCE OF " + e.name("runtime.arraymark") + ".\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `arrayView: source is not array storage`.\nENDIF.\nENDIF.")
		b.line("CREATE OBJECT " + n + ".\n" + n + "->source = " + a + ".\n" + n + "->load( ).")
		return
	}
	b.line("CLEAR " + n + ".")
	b.line("IF " + a + " IS BOUND.\nIF " + a + "->tag = " + d + "=>tag_ref.\nTRY.\n" + n + " ?= " + a + "->oval.\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: arrayView: incompatible native array reference`.\nENDTRY.\nELSEIF " + a + "->tag = " + d + "=>tag_array.\nCREATE OBJECT " + n + ".\n" + n + "->source = " + a + ".\nELSE.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: arrayView: source is neither array graph nor reference`.\nENDIF.\n" + n + "->load( ).")
	// Validate every present element at publication and again on subsequent reads.
	if t.Args[0].IsRef() && t.Args[0].Name != hir.RootObject {
		row := b.temp(hir.Ref(hir.RootObject))
		v := b.temp(t.Args[0])
		b.line("LOOP AT " + n + "->items INTO " + row + ".")
		b.arrayElement(v, row, t.Args[0])
		b.line("ENDLOOP.")
	}
	b.line("ENDIF.")
}
func (b *body) arrayElement(n, row string, t hir.Type) {
	if t.Kind == hir.Optional {
		t = t.Args[0]
	}
	if t.Name == hir.RootObject {
		b.line(n + " = " + row + ".")
		return
	}
	if !b.e.arrayViews && t.Kind != hir.Dynamic {
		b.line(n + " ?= " + row + ".")
		return
	}
	d := b.temp(hir.T(hir.Dynamic))
	b.line("CLEAR " + n + ".\nIF " + row + " IS INSTANCE OF " + b.e.name("runtime.dynamic") + ".\n" + d + " ?= " + row + ".")
	if t.Kind == hir.Dynamic {
		b.line(n + " = " + d + ".")
	} else {
		b.liveUnbox(n, d, t)
	}
	b.line("ELSE.")
	if t.Kind == hir.Dynamic {
		b.line("IF " + row + " IS INSTANCE OF " + b.e.name("runtime.classvalue") + ".")
		cv := b.rawTemp("REF TO " + b.e.name("runtime.classvalue"))
		b.line(cv + " ?= " + row + ".")
		b.boxDynamic(n, cv, hir.T(hir.ClassValue))
		b.line("ELSE.")
		b.boxDynamic(n, row, hir.Ref(hir.RootObject))
		b.line("ENDIF.")
	} else {
		b.line(n + " ?= " + row + ".")
	}
	b.line("ENDIF.")
}
