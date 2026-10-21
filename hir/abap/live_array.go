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
	// Native arrays expose the same checked element storage as graph arrays.
	// Refresh a private read buffer; identity and every write stay at source.
	adapter := b.rawTemp("REF TO " + storage)
	length := b.temp(hir.T(hir.Dynamic))
	index := b.temp(hir.T(hir.I32))
	key := b.temp(hir.T(hir.String))
	item := b.temp(hir.T(hir.Dynamic))
	value := b.temp(t.Args[0])
	b.line("IF source->tag = " + d + "=>tag_ref.\n" + adapter + " ?= source->oval.\n" + length + " = " + adapter + "->get( `length` ).\n" + index + " = 0.\nWHILE " + index + " < " + length + "->nval.\n" + key + " = |{ " + index + " }|.\nCLEAR " + value + ".\nIF " + adapter + "->has( " + key + " ) = abap_false.\nINSERT " + index + " + 1 INTO TABLE missing.\nELSE.\n" + item + " = " + adapter + "->get( " + key + " ).\nIF " + item + " IS BOUND.")
	if t.Args[0].Name == hir.RootObject {
		b.line("IF " + item + "->tag = " + d + "=>tag_ref.\n" + value + " = " + item + "->as_ref( ).\nELSE.\n" + value + " = " + item + ".\nENDIF.")
	} else {
		b.liveUnbox(value, item, t.Args[0])
	}
	b.line("ELSE.")
	if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional || (t.Args[0].Kind == hir.InterfaceRef && e.structuralUnion(t.Args[0]) != "" || e.p.StructuralArrayUnion(t.Args[0])) {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: native load: undefined cannot inhabit required element`.")
	}
	b.line("ENDIF.\nENDIF.\nAPPEND " + value + " TO items.\n" + index + " = " + index + " + 1.\nENDWHILE.\nRETURN.\nENDIF.")

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
	if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional || (t.Args[0].Kind == hir.InterfaceRef && e.structuralUnion(t.Args[0]) != "" || e.p.StructuralArrayUnion(t.Args[0])) {
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
	adapter = b.rawTemp("REF TO " + storage)
	length = b.temp(hir.T(hir.Dynamic))
	index = b.temp(hir.T(hir.I32))
	key = b.temp(hir.T(hir.String))
	item = b.temp(hir.T(hir.Dynamic))
	value = b.temp(t.Args[0])
	b.line("IF source->tag = " + d + "=>tag_ref.\n" + adapter + " ?= source->oval.\nCREATE OBJECT " + length + ".\n" + length + "->tag = " + d + "=>tag_number.\n" + length + "->nval = lines( items ).\n" + adapter + "->put( p0 = `length` p1 = " + length + " ).\n" + index + " = 0.\nLOOP AT items INTO " + value + ".\n" + key + " = |{ " + index + " }|.\nREAD TABLE missing WITH TABLE KEY table_line = " + index + " + 1 TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.\n" + adapter + "->delete( " + key + " ).\nELSE.\nCLEAR " + item + ".")
	if wideInteger {
		b.line("RAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: native save: i64 element boxing unavailable`.")
	} else {
		b.boxDynamic(item, value, t.Args[0])
	}
	b.line(adapter + "->put( p0 = " + key + " p1 = " + item + " ).\nENDIF.\n" + index + " = " + index + " + 1.\nENDLOOP.\nRETURN.\nENDIF.")
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
	code.WriteString("METHOD " + storage + "~source.\nresult = source.\nENDMETHOD.\nMETHOD " + storage + "~structural.\nENDMETHOD.\n")
	for _, op := range []string{"has", "delete"} {
		b = makeBody()
		node = b.temp(hir.T(hir.Dynamic))
		idx = b.temp(hir.T(hir.I32))
		if op == "has" {
			b.line("IF p0 = `length`.\nresult = abap_true.\nRETURN.\nENDIF.")
		}
		b.line("CREATE OBJECT " + node + ".\nTRY.\n" + idx + " = " + node + "->array_index( p0 ).\nCATCH cx_sy_range_out_of_bounds.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: native array property operation requires an index`.\nENDTRY.\nresult = " + op + "( " + idx + " ).")
		methodCode := b.code.String()
		if op == "has" {
			methodCode = strings.Replace(methodCode, "RAISE EXCEPTION TYPE "+trap+" EXPORTING reason = `live_array.go: native array property operation requires an index`.", "RETURN.", 1)
		}
		code.WriteString("METHOD " + storage + "~" + op + ".\n" + b.constantDeclarations() + methodCode + "ENDMETHOD.\n")
	}
	b = makeBody()
	idx = b.temp(hir.T(hir.I32))
	node = b.temp(hir.T(hir.Dynamic))
	v = b.temp(t.Args[0])
	b.line("load( ).\nIF p0 = `length`.\nCREATE OBJECT result.\nresult->tag = " + d + "=>tag_number.\nresult->nval = lines( items ).\nRETURN.\nENDIF.\nCREATE OBJECT " + node + ".\nTRY.\n" + idx + " = " + node + "->array_index( p0 ).\nCATCH cx_sy_range_out_of_bounds.\nRETURN.\nENDTRY.\nIF has( " + idx + " ) = abap_false.\nRETURN.\nENDIF.\nREAD TABLE items INDEX " + idx + " + 1 INTO " + v + ".")
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
	// Resizing preserves the original array object, including holes on growth.
	b.line("IF p0 = `length`.\nIF p1 IS NOT BOUND OR p1->tag <> " + d + "=>tag_number.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: length requires a number`.\nENDIF.\nIF p1->nval < 0 OR p1->nval >= 2147483647 OR p1->nval <> trunc( p1->nval ).\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\n" + idx + " = p1->nval.\nload( ).\nIF " + idx + " < lines( items ).\n" + idx + " = " + idx + " + 1.\nDELETE items FROM " + idx + ".\n" + idx + " = " + idx + " - 1.\nDELETE missing WHERE table_line > " + idx + ".\nELSE.\nallocate( " + idx + " ).\nENDIF.\nsave( ).\nRETURN.\nENDIF.")
	b.line("CREATE OBJECT " + node + ".\n" + idx + " = " + node + "->array_index( p0 ).")
	// Preserve undefined/reference identity; required scalar slots refuse undefined.
	b.line("IF p1 IS BOUND.")
	if t.Args[0].Name == hir.RootObject {
		b.line("IF p1->tag = " + d + "=>tag_ref.\n" + v + " = p1->as_ref( ).\nELSE.\n" + v + " = p1.\nENDIF.")
	} else {
		b.liveUnbox(v, "p1", t.Args[0])
	}
	b.line("ELSE.")
	if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional || (t.Args[0].Kind == hir.InterfaceRef && e.structuralUnion(t.Args[0]) != "" || e.p.StructuralArrayUnion(t.Args[0])) {
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

// Explicit conversions require array storage. A data-field read may still
// observe an absent property through the ordinary live view below.
func (b *body) requiredArrayView(t hir.Type, n, a string) {
	b.line("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + b.e.name("view.refusal") + " EXPORTING reason = `arrayView: undefined is not array storage`.\nENDIF.")
	b.arrayView(t, n, a)
}
func (b *body) arrayView(t hir.Type, n, a string) {
	e := b.e
	e.liveRuntime()
	trap, d := e.name("view.refusal"), e.name("runtime.dynamic")
	if t.Args[0].Kind == hir.Dynamic {
		e.isArrayUsed = true
		b.line("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `arrayView: undefined is not array storage`.\nENDIF.")
		b.line("IF " + a + "->tag <> " + d + "=>tag_array.\nIF " + a + "->tag <> " + d + "=>tag_ref OR " + a + "->oval IS NOT INSTANCE OF " + e.name("runtime.arraymark") + ".\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `arrayView: source is not array storage`.\nENDIF.\nENDIF.")
		b.line("CREATE OBJECT " + n + ".\n" + n + "->source = " + a + ".\n" + n + "->load( ).")
		return
	}
	b.line("CLEAR " + n + ".")
	e.isArrayUsed = true
	b.line("IF " + a + " IS BOUND.\nIF " + a + "->tag = " + d + "=>tag_ref.\n" + castOrElse(n, a+"->oval", refClass(e.typ(t)), "IF "+a+"->oval IS NOT INSTANCE OF "+e.name("runtime.arraymark")+".\nRAISE EXCEPTION TYPE "+trap+" EXPORTING reason = `live_array.go: arrayView: reference is not array storage`.\nENDIF.\nCREATE OBJECT "+n+".\n"+n+"->source = "+a+".") + "\nELSEIF " + a + "->tag = " + d + "=>tag_array.\nCREATE OBJECT " + n + ".\n" + n + "->source = " + a + ".\nELSE.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `live_array.go: arrayView: source is neither array graph nor reference`.\nENDIF.\n" + n + "->load( ).")
	// Validate every present element at publication and again on subsequent reads.
	if t.Args[0].IsRef() && t.Args[0].Name != hir.RootObject {
		row := b.temp(hir.Ref(hir.RootObject))
		v := b.temp(t.Args[0])
		b.line("LOOP AT " + n + "->items INTO " + row + ".")
		b.line("READ TABLE " + n + "->missing WITH TABLE KEY table_line = sy-tabix TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.")
		b.arrayElement(v, row, t.Args[0])
		b.line("ENDIF.\nENDLOOP.")
	}
	b.line("ENDIF.")
}
func (b *body) arrayElement(n, row string, t hir.Type) {
	if t.Kind == hir.Optional {
		if t.Args[0].Kind == hir.InterfaceRef && b.e.structuralUnion(t.Args[0]) != "" || b.e.p.StructuralArrayUnion(t.Args[0]) {
			b.line("CLEAR " + n + ".\nIF " + row + " IS BOUND.")
			b.arrayElement(n, row, t.Args[0])
			b.line("ENDIF.")
			return
		}
		t = t.Args[0]
	}
	if t.Kind == hir.InterfaceRef && b.e.structuralUnion(t) != "" || b.e.p.StructuralArrayUnion(t) {
		b.line("IF " + row + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + b.e.name("view.refusal") + " EXPORTING reason = `structural union array requires a present record`.\nENDIF.")
		d := b.temp(hir.T(hir.Dynamic))
		b.liveBox(d, row, t)
		b.liveUnbox(n, d, t)
		return
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
	} else if t.Kind == hir.UnionRef {
		// Erased storage holds the payload object itself, or a foreign record.
		b.unionFromObject(n, row, t)
	} else if t.Kind == hir.ClassRef && b.e.liveClasses[t.Name] {
		// An admitted structural layout may read a different native data layout.
		// Rebox the original slot and use the checked live converter, not a nominal cast.
		b.liveBox(d, row, hir.Ref(hir.RootObject))
		b.liveUnbox(n, d, t)
	} else {
		b.line(n + " ?= " + row + ".")
	}
	b.line("ENDIF.")
}
