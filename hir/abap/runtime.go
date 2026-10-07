package abap

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"strings"
)

func (e *emitter) runtime(t hir.Type) {
	id := "runtime." + t.String()
	if e.types[id] {
		return
	}
	e.types[id] = true
	name := e.name(id)
	var def, impl strings.Builder
	line := func(s string) { def.WriteString(s + "\n") }
	method := func(n, s string) { impl.WriteString("METHOD " + n + ".\n" + s + "ENDMETHOD.\n") }
	switch t.Kind {
	case hir.Optional:
		line("DATA value TYPE " + e.typ(t.Args[0]) + ".")
		line("DATA has TYPE abap_bool.")
	case hir.Array:
		elem := e.typ(t.Args[0])
		opt := hir.T(hir.Optional, t.Args[0])
		line("TYPES items_type TYPE STANDARD TABLE OF " + elem + " WITH DEFAULT KEY.")
		line("DATA items TYPE items_type.")
		line("METHODS push IMPORTING p0 TYPE " + elem + " RETURNING VALUE(result) TYPE i.")
		method("push", "APPEND p0 TO items.\nresult = lines( items ).\n")
		line("METHODS length RETURNING VALUE(result) TYPE i.")
		method("length", "result = lines( items ).\n")
		line("METHODS get IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE " + e.typ(opt) + ".")
		code := "DATA idx TYPE i.\nDATA val TYPE " + elem + ".\nIF p0 < 0.\nRETURN.\nENDIF.\nidx = p0 + 1.\nREAD TABLE items INDEX idx INTO val.\nIF sy-subrc = 0.\n"
		if t.Args[0].IsRef() {
			code += "result = val.\n"
		} else {
			code += "CREATE OBJECT result.\nresult->has = abap_true.\nresult->value = val.\n"
		}
		method("get", code+"ENDIF.\n")
	case hir.OrderedMap, hir.OrderedSet:
		key := e.typ(t.Args[0])
		value := hir.T(hir.Bool)
		if t.Kind == hir.OrderedMap {
			value = t.Args[1]
		}
		vt := e.typ(value)
		line("TYPES: BEGIN OF entry, k TYPE " + key + ", v TYPE " + vt + ", END OF entry.")
		line("TYPES entries_type TYPE STANDARD TABLE OF entry WITH DEFAULT KEY.")
		line("DATA entries TYPE entries_type.")
		line("METHODS size RETURNING VALUE(result) TYPE i.")
		method("size", "result = lines( entries ).\n")
		line("METHODS has IMPORTING p0 TYPE " + key + " RETURNING VALUE(result) TYPE abap_bool.")
		method("has", "READ TABLE entries WITH KEY k = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.\nresult = abap_true.\nENDIF.\n")
		op := "add"
		params := ""
		val := "abap_true"
		if t.Kind == hir.OrderedMap {
			op = "set"
			params = " p1 TYPE " + vt
			val = "p1"
		}
		line("METHODS " + op + " IMPORTING p0 TYPE " + key + params + " RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method(op, "DATA row TYPE entry.\nREAD TABLE entries WITH KEY k = p0 INTO row.\nIF sy-subrc = 0.\nrow-v = "+val+".\nMODIFY entries FROM row INDEX sy-tabix.\nELSE.\nrow-k = p0.\nrow-v = "+val+".\nAPPEND row TO entries.\nENDIF.\nresult = me.\n")
		if t.Kind == hir.OrderedMap {
			opt := e.typ(hir.T(hir.Optional, value))
			line("METHODS get IMPORTING p0 TYPE " + key + " RETURNING VALUE(result) TYPE " + opt + ".")
			code := "DATA row TYPE entry.\nREAD TABLE entries WITH KEY k = p0 INTO row.\nIF sy-subrc = 0.\n"
			if value.IsRef() {
				code += "result = row-v.\n"
			} else {
				code += "CREATE OBJECT result.\nresult->has = abap_true.\nresult->value = row-v.\n"
			}
			method("get", code+"ENDIF.\n")
		}
		op = "values"
		if t.Kind == hir.OrderedMap {
			op = "keys"
		}
		line("METHODS " + op + " RETURNING VALUE(result) TYPE " + e.typ(hir.T(hir.Array, t.Args[0])) + ".")
		method(op, "DATA row TYPE entry.\nCREATE OBJECT result.\nLOOP AT entries INTO row.\nAPPEND row-k TO result->items.\nENDLOOP.\n")
	}
	e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\n" + def.String() + "ENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
}
func (e *emitter) instanceHelper(owner string) string {
	name := e.name("instanceof." + owner)
	file := name + ".clas.abap"
	if _, ok := e.files[file]; ok {
		return name
	}
	e.files[file] = fmt.Sprintf(`CLASS %s DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CLASS-METHODS test IMPORTING value TYPE REF TO object RETURNING VALUE(result) TYPE abap_bool.
ENDCLASS.
CLASS %s IMPLEMENTATION.
METHOD test.
DATA narrowed TYPE REF TO %s.
IF value IS BOUND.
TRY.
narrowed ?= value.
result = abap_true.
CATCH cx_sy_move_cast_error.
result = abap_false.
ENDTRY.
ENDIF.
ENDMETHOD.
ENDCLASS.
`, name, name, e.name(owner))
	return name
}
func (e *emitter) exception(t hir.Type) string {
	name := e.name("exception." + t.String())
	file := name + ".clas.abap"
	if _, ok := e.files[file]; ok {
		return name
	}
	e.files[file] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA payload TYPE " + e.typ(t) + ".\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
	return name
}
