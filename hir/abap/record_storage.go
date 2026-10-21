package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) recordGraphRuntime() {
	d, adapter, trap := e.name("runtime.dynamic"), e.name("view.storage"), e.name("view.refusal")
	file := d + ".clas.abap"
	text := e.files[file]
	text = strings.Replace(text, "METHODS get IMPORTING", "METHODS has IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nMETHODS delete IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nMETHODS get IMPORTING", 1)
	code := `METHOD has.
IF tag = tag_object.
READ TABLE entries WITH KEY k = p0 TRANSPORTING NO FIELDS.
result = xsdbool( sy-subrc = 0 ).
ELSEIF tag = tag_array.
IF p0 = 'length'.
result = abap_true.
RETURN.
ENDIF.
DATA array_index_value TYPE i.
TRY.
array_index_value = array_index( p0 ) + 1.
CATCH cx_sy_range_out_of_bounds.
RETURN.
ENDTRY.
IF array_index_value <= lines( items ).
READ TABLE missing WITH TABLE KEY table_line = array_index_value TRANSPORTING NO FIELDS.
result = xsdbool( sy-subrc <> 0 ).
ENDIF.
ELSEIF tag = tag_ref.
DATA adapter TYPE REF TO $S.
TRY.
adapter ?= oval.
CATCH cx_sy_move_cast_error.
RAISE EXCEPTION TYPE $E EXPORTING reason = 'record_storage.go: dynamic.has: reference lacks view storage'.
ENDTRY.
result = adapter->has( p0 ).
ELSE.
RAISE EXCEPTION TYPE $E EXPORTING reason = 'record_storage.go: dynamic.has: source is not object or reference'.
ENDIF.
ENDMETHOD.
METHOD delete.
IF tag = tag_object.
DELETE entries WHERE k = p0.
result = abap_true.
ELSEIF tag = tag_ref.
DATA adapter TYPE REF TO $S.
TRY.
adapter ?= oval.
CATCH cx_sy_move_cast_error.
RAISE EXCEPTION TYPE $E EXPORTING reason = 'record_storage.go: dynamic.delete: reference lacks view storage'.
ENDTRY.
result = adapter->delete( p0 ).
ELSE.
RAISE EXCEPTION TYPE $E EXPORTING reason = 'record_storage.go: dynamic.delete: source is not object or reference'.
ENDIF.
ENDMETHOD.
`
	if e.recordDelete {
		fallback := e.nativeRecordDeleteRuntime()
		start := strings.Index(code, "METHOD delete.")
		code = code[:start] + strings.Replace(code[start:], "CATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE $E EXPORTING reason = 'record_storage.go: dynamic.delete: reference lacks view storage'.", "CATCH cx_sy_move_cast_error.\nresult = "+fallback+"=>run( p0 = oval key = p0 ).\nRETURN.", 1)
	}
	text = strings.Replace(text, "METHOD as_ref.\n", "METHOD as_ref.\nIF tag = tag_object OR tag = tag_array.\nresult = me.\nRETURN.\nENDIF.\n", 1)
	text = strings.TrimSuffix(text, "ENDCLASS.\n") + strings.NewReplacer("$S", adapter, "$E", trap).Replace(code) + "ENDCLASS.\n"
	e.files[file] = text
}
func (b *body) markOwnKey(receiver, key string) {
	bag := receiver + "->" + b.e.name("view.ownKeys")
	b.line("IF " + bag + " IS BOUND.\n" + bag + "->put( p0 = `" + strings.ReplaceAll(key, "`", "``") + "` p1 = " + b.temp(hir.T(hir.Dynamic)) + " ).\nENDIF.")
}
func (b *body) attachOwnKeys(x *hir.Expr, n string) {
	c := b.e.classBy(x.Type.Name)
	if !b.e.types["runtime.liveView"] || c == nil || !c.Structural {
		return
	}
	if x.OwnKeys == nil {
		b.factoryOwnKeys(x, n)
		return
	}
	bag := n + "->" + b.e.name("view.ownKeys")
	b.line("CREATE OBJECT " + bag + ".\n" + bag + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_object.")
	for _, key := range *x.OwnKeys {
		b.markOwnKey(n, key)
	}
}
func (e *emitter) nativeStructuralPresence(out *strings.Builder, c *hir.Class) {
	storage, bag, trap := e.name("view.storage"), "me->"+e.name("view.ownKeys"), e.name("view.refusal")
	fmt.Fprintf(out, "METHOD %s~has.\n", storage)
	if e.liveRoot(c) {
		fmt.Fprintf(out, "IF me->%s IS BOUND.\nresult = me->%s->has( p0 ).\nRETURN.\nENDIF.\n", e.name("view.source"), e.name("view.source"))
	}
	fmt.Fprintf(out, "IF %s IS NOT BOUND.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: structural.has: own-key metadata absent`.\nENDIF.\nresult = %s->has( p0 ).\nIF result = abap_false AND me->%s IS BOUND.\nresult = me->%s->has( p0 ).\nENDIF.\nENDMETHOD.\n", bag, trap, bag, e.name("view.extras"), e.name("view.extras"))
	fmt.Fprintf(out, "METHOD %s~delete.\nDATA ignored TYPE abap_bool.\n", storage)
	if e.liveRoot(c) {
		fmt.Fprintf(out, "IF me->%s IS BOUND.\nresult = me->%s->delete( p0 ).\nRETURN.\nENDIF.\n", e.name("view.source"), e.name("view.source"))
	}
	out.WriteString("CASE p0.\n")
	for _, f := range c.Fields {
		if f.Static {
			continue
		}
		fmt.Fprintf(out, "WHEN `%s`.\n", strings.ReplaceAll(f.Name, "`", "``"))
		typ := e.semanticFieldType(c, f)
		if prototypeProperty(f.Name) || typ.Kind != hir.Optional && typ.Kind != hir.Dynamic {
			fmt.Fprintf(out, "RAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: structural.delete: required or prototype slot cannot retain absence`.\n", trap)
		} else {
			fmt.Fprintf(out, "CLEAR me->%s.\n", e.member(f.Name))
			if e.recordDelete {
				fmt.Fprintf(out, "INSERT p0 INTO TABLE me->%s.\n", e.name("record.deleted"))
			}
		}
	}
	out.WriteString("WHEN OTHERS.\n")
	if e.recordDelete {
		fmt.Fprintf(out, "ignored = %s=>run( p0 = me key = p0 ).\n", e.nativeRecordDeleteRuntime())
	}
	fmt.Fprintf(out, "IF me->%s IS BOUND.\nignored = me->%s->delete( p0 ).\nENDIF.\nENDCASE.\nIF %s IS BOUND.\nignored = %s->delete( p0 ).\nENDIF.\nresult = abap_true.\nENDMETHOD.\n", e.name("view.extras"), e.name("view.extras"), bag, bag)
}

// Root-object lookup accepts proven structural storage and arrays, without
// granting a nominal class layout. Reads use the same presence proof as has.
func (b *body) erasedPropertyLookup(x *hir.Expr, n, a, key string) {
	e := b.e
	e.liveRuntime()
	e.isArrayUsed = true
	d := b.temp(hir.T(hir.Dynamic))
	adapter := b.rawTemp("REF TO " + e.name("view.storage"))
	present := b.temp(hir.T(hir.Bool))
	inherited := b.temp(hir.T(hir.Bool))
	trap := e.name("view.refusal")
	b.line("CLEAR " + d + ".\nCLEAR " + adapter + ".\nIF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `object lookup requires a receiver`.\nENDIF.")
	b.line("TRY.\n" + d + " ?= " + a + ".\nCATCH cx_sy_move_cast_error.\nENDTRY.")
	b.line("IF " + d + " IS NOT BOUND.\nTRY.\n" + adapter + " ?= " + a + ".\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `object lookup lacks structural storage`.\nENDTRY.\n" + d + " = " + adapter + "->source( ).\nENDIF.")
	if len(e.p.Unions) > 0 {
		// A union payload reaches erased object code boxed by reference.
		native := b.rawTemp("REF TO object")
		b.line("IF " + d + " IS BOUND AND " + d + "->tag = " + e.name("runtime.dynamic") + "=>tag_ref.\n" + native + " = " + d + "->oval.\nIF " + native + " IS NOT INSTANCE OF " + e.name("view.storage") + ".\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `object lookup lacks structural storage`.\nENDIF.\n" + adapter + " ?= " + native + ".\n" + d + " = " + adapter + "->source( ).\nENDIF.")
	}
	b.line("IF " + d + " IS NOT BOUND.\nIF " + adapter + "->structural( ) <> abap_true AND " + a + " IS NOT INSTANCE OF " + e.name("runtime.arraymark") + ".\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `object lookup refuses nominal storage`.\nENDIF.\n" + present + " = " + adapter + "->has( " + key + " ).\nELSE.\nIF " + d + "->tag <> " + e.name("runtime.dynamic") + "=>tag_object AND " + d + "->tag <> " + e.name("runtime.dynamic") + "=>tag_array.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `object lookup requires structural object or array`.\nENDIF.\n" + present + " = " + d + "->has( " + key + " ).\nENDIF.")
	clauses := []string{}
	for _, v := range []string{"constructor", "__proto__", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
		clauses = append(clauses, key+" = `"+v+"`")
	}
	b.line(inherited + " = xsdbool( " + strings.Join(clauses, " OR ") + " ).")
	arrayClauses := []string{}
	for _, v := range []string{"at", "concat", "copyWithin", "fill", "find", "findIndex", "findLast", "findLastIndex", "lastIndexOf", "pop", "push", "reverse", "shift", "unshift", "slice", "sort", "splice", "includes", "indexOf", "join", "keys", "entries", "values", "forEach", "filter", "flat", "flatMap", "map", "every", "some", "reduce", "reduceRight", "toReversed", "toSorted", "toSpliced", "with"} {
		arrayClauses = append(arrayClauses, key+" = `"+v+"`")
	}
	b.line("IF ( " + d + " IS BOUND AND " + d + "->tag = " + e.name("runtime.dynamic") + "=>tag_array ) OR " + a + " IS INSTANCE OF " + e.name("runtime.arraymark") + ".\nIF " + strings.Join(arrayClauses, " OR ") + ".\n" + inherited + " = abap_true.\nENDIF.\nENDIF.")
	if x.Op == "object.get" {
		b.line("CLEAR " + n + ".\nIF " + present + " = abap_true.\nIF " + d + " IS BOUND.\n" + n + " = " + d + "->get( " + key + " ).\nELSE.\n" + n + " = " + adapter + "->get( " + key + " ).\nENDIF.\nELSEIF " + inherited + " = abap_true.\nRAISE EXCEPTION TYPE " + trap + " EXPORTING reason = `erased prototype reads remain unsupported`.\nENDIF.")
	} else if x.Op == "object.has" {
		b.line(n + " = xsdbool( " + present + " = abap_true OR " + inherited + " = abap_true ).")
	} else {
		b.line(n + " = " + present + ".")
	}
}

// Nominal optional slots can retain absence; required slots explicitly refuse.
// Keep a separate deletion mask because an undefined value is still present.
func (e *emitter) nativeRecordDeleteRuntime() string {
	return e.nativeDeletePart(false)
}

func (e *emitter) nativeDeletePart(testPart bool) string {
	name := e.name("runtime.nativeDelete")
	if testPart {
		name = "zcl_lnt_test_delete"
	}
	if e.types[name] {
		return name
	}
	e.types[name] = true
	var code strings.Builder
	fmt.Fprintf(&code, "CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS run IMPORTING p0 TYPE REF TO object key TYPE string RETURNING VALUE(result) TYPE abap_bool.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD run.\nIF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: nativeDelete: receiver is undefined`.\nENDIF.\n", name, name, e.name("view.refusal"))
	if e.testPackages && !testPart {
		factory := e.name("runtime.classvalue.factory")
		fmt.Fprintf(&code, "IF %s=>extension IS BOUND.\nDATA handled TYPE abap_bool.\n%s=>extension->delete( EXPORTING p0 = p0 key = key IMPORTING handled = handled result = result ).\nIF handled = abap_true.\nRETURN.\nENDIF.\nENDIF.\n", factory, factory)
	}
	for i, c := range e.classesByDepth() {
		if e.testPackages && testDeclaration(c) != testPart {
			continue
		}
		fmt.Fprintf(&code, "IF p0 IS INSTANCE OF %s.\nDATA native%d TYPE REF TO %s.\nnative%d ?= p0.\n", e.name(c.Name), i, e.name(c.Name), i)
		seen := map[string]bool{}
		caseOpened := false
		for base := c; base != nil; base = e.classBy(base.Super) {
			for _, f := range base.Fields {
				if f.Static || seen[f.Name] {
					continue
				}
				seen[f.Name] = true
				if !caseOpened {
					code.WriteString("CASE key.\n")
					caseOpened = true
				}
				fmt.Fprintf(&code, "WHEN `%s`.\n", strings.ReplaceAll(f.Name, "`", "``"))
				typ := e.semanticFieldType(base, f)
				if prototypeProperty(f.Name) || typ.Kind != hir.Optional && typ.Kind != hir.Dynamic {
					fmt.Fprintf(&code, "RAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: nativeDelete: required or prototype slot cannot retain absence`.\n", e.name("view.refusal"))
				} else {
					fmt.Fprintf(&code, "CLEAR native%d->%s.\nINSERT key INTO TABLE native%d->%s.\n", i, e.member(f.Name), i, e.name("record.deleted"))
				}
			}
		}
		if caseOpened {
			code.WriteString("ENDCASE.\n")
		}
		code.WriteString("result = abap_true.\nRETURN.\nENDIF.\n")
	}
	if testPart {
		text := code.String()
		text = strings.Replace(text, " RETURNING VALUE(result) TYPE abap_bool.", " EXPORTING handled TYPE abap_bool result TYPE abap_bool.", 1)
		text = strings.ReplaceAll(text, "result = abap_true.\nRETURN.", "handled = abap_true.\nresult = abap_true.\nRETURN.")
		code.Reset()
		code.WriteString(text)
		code.WriteString("ENDMETHOD.\nENDCLASS.\n")
	} else {
		fmt.Fprintf(&code, "RAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: nativeDelete: native class layout unknown`.\nENDMETHOD.\nENDCLASS.\n", e.name("view.refusal"))
	}
	e.files[name+".clas.abap"] = code.String()
	return name
}

// These own slots can reveal inherited values after deletion. The nominal
// table does not retain that second slot, so refuse rather than hide it.
func prototypeProperty(key string) bool {
	switch key {
	case "constructor", "__proto__", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__":
		return true
	}
	return false
}
