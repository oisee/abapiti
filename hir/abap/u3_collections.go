package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

func (b *body) u3CollectionRuntime(x *hir.Expr, n string) bool {
	switch x.Op {
	case "array.spread0":
		a := b.expr(x.X)
		if b.e.arrayViews {
			b.line(a + "->load( ).")
		}
		b.line("CREATE OBJECT " + n + ".")
		row := b.temp(arrayStorage(x.X.Type).Args[0])
		index := b.temp(hir.T(hir.I32))
		value := b.temp(arrayStorage(x.Type).Args[0])
		b.line("LOOP AT " + a + "->items INTO " + row + ".")
		b.line(index + " = sy-tabix.")
		b.line("CLEAR " + value + ".")
		b.line("READ TABLE " + a + "->missing WITH TABLE KEY table_line = " + index + " TRANSPORTING NO FIELDS.")
		b.line("IF sy-subrc <> 0.")
		b.line(value + " = " + b.convert(row, arrayStorage(x.X.Type).Args[0], arrayStorage(x.Type).Args[0]) + ".")
		b.line("ENDIF.")
		b.line("APPEND " + value + " TO " + n + "->items.")
		b.line("ENDLOOP.")
		return true
	case "collection.has", "collection.get":
		a, key := b.expr(x.X), b.expr(x.Args[0])
		keysType := hir.T(hir.Array, x.X.Type.Args[0])
		keys := b.temp(keysType)
		method := "keys"
		if x.X.Type.Kind == hir.OrderedSet {
			method = "values"
		}
		b.line(keys + " = " + a + "->" + method + "( ).")
		row := b.temp(arrayStorage(keysType).Args[0])
		box := b.temp(hir.T(hir.Dynamic))
		b.line("CLEAR " + n + ".")
		b.line("LOOP AT " + keys + "->items INTO " + row + ".")
		native := row
		if x.X.Type.Args[0].IsRef() {
			native = b.temp(x.X.Type.Args[0])
			b.line(native + " ?= " + row + ".")
		}
		b.boxDynamic(box, native, x.X.Type.Args[0])
		equal := b.temp(hir.T(hir.Bool))
		b.line("CLEAR " + equal + ".")
		b.line("IF " + box + " IS NOT BOUND AND " + key + " IS NOT BOUND.\n" + equal + " = abap_true.")
		b.line("ELSEIF " + box + " IS BOUND AND " + key + " IS BOUND.")
		b.line(equal + " = " + box + "->strict_equals( " + key + " ).")
		d := b.e.name("runtime.dynamic")
		b.line("IF " + box + "->tag = " + d + "=>tag_number AND " + key + "->tag = " + d + "=>tag_number.")
		b.line("IF " + box + "->number_special = 1 AND " + key + "->number_special = 1.\n" + equal + " = abap_true.\nENDIF.\nENDIF.\nENDIF.")
		b.line("IF " + equal + " = abap_true.")
		if x.Op == "collection.has" {
			b.line(n + " = abap_true.")
		} else {
			b.line(n + " = " + a + "->get( " + native + " ).")
		}
		b.line("EXIT.\nENDIF.\nENDLOOP.")
		return true
	}
	return false
}

func (e *emitter) setIterationUsed() bool {
	return e.usesReviewedOp("set.iterationValues")
}
func (e *emitter) setIterationMethod(t hir.Type, method, code string) string {
	if t.Kind != hir.OrderedSet || !e.setIterationUsed() {
		return code
	}
	switch method {
	case "add":
		appendValue := "APPEND p0 TO iteration->items.\n"
		if t.Args[0].Kind == hir.Number {
			appendValue = "IF p0 = 0.\nAPPEND INITIAL LINE TO iteration->items.\nELSE.\n" + appendValue + "ENDIF.\n"
		}
		return strings.Replace(code, "APPEND VALUE #( k = p0 v = abap_true ) TO entries.", "APPEND VALUE #( k = p0 v = abap_true ) TO entries.\nIF iteration IS BOUND.\n"+appendValue+"ENDIF.", 1)
	case "delete":
		hook := `IF iteration IS BOUND.
DATA cursor TYPE i.
DATA item TYPE ` + e.typ(arrayStorage(hir.T(hir.Array, t.Args[0])).Args[0]) + `.
DATA ignored TYPE abap_bool.
cursor = lines( iteration->items ).
WHILE cursor > 0.
IF iteration->has( cursor - 1 ) = abap_true.
READ TABLE iteration->items INDEX cursor INTO item.
IF item = p0.
ignored = iteration->delete( cursor - 1 ).
EXIT.
ENDIF.
ENDIF.
cursor = cursor - 1.
ENDWHILE.
ENDIF.
`
		return strings.Replace(code, "result = abap_true.\n", hook+"result = abap_true.\n", 1)
	}
	return code
}
func (e *emitter) setIterationDefinition(t hir.Type) (string, string) {
	if t.Kind != hir.OrderedSet || !e.setIterationUsed() {
		return "", ""
	}
	arr := e.typ(hir.T(hir.Array, t.Args[0]))
	def := "DATA iteration TYPE " + arr + ".\nMETHODS iterationValues RETURNING VALUE(result) TYPE " + arr + ".\nMETHODS clear_iteration.\n"
	impl := `METHOD iterationValues.
IF iteration IS NOT BOUND.
iteration = values( ).
ENDIF.
result = iteration.
ENDMETHOD.
METHOD clear_iteration.
DATA cursor TYPE i.
DATA ignored TYPE abap_bool.
IF iteration IS BOUND.
WHILE cursor < lines( iteration->items ).
ignored = iteration->delete( cursor ).
cursor = cursor + 1.
ENDWHILE.
ENDIF.
CLEAR entries.
ENDMETHOD.
`
	if t.Args[0].Kind == hir.Number {
		impl = strings.Replace(impl, "iteration = values( ).\n", "iteration = values( ).\nLOOP AT iteration->items ASSIGNING FIELD-SYMBOL(<zero>).\nIF <zero> = 0.\n<zero> = 0.\nENDIF.\nENDLOOP.\n", 1)
	}
	return def, impl
}
