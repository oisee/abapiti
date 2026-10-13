package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Missing is independent of the stored value: an initial optional/reference
// can be either a hole or a present undefined value.
func (e *emitter) sparseArrayDefinition(t hir.Type, name string) string {
	elem := e.typ(t.Args[0])
	return "TYPES missing_type TYPE SORTED TABLE OF i WITH UNIQUE KEY table_line.\nDATA missing TYPE missing_type.\nMETHODS has IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE abap_bool.\nMETHODS delete IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE abap_bool.\nMETHODS put IMPORTING p0 TYPE i p1 TYPE " + elem + ".\nMETHODS allocate IMPORTING p0 TYPE i.\nMETHODS dense.\n"
}
func (e *emitter) sparseArrayImplementation(t hir.Type) string {
	elem := e.typ(t.Args[0])
	return `METHOD has.
IF p0 < 0 OR p0 >= lines( items ).
RETURN.
ENDIF.
DATA idx TYPE i.
idx = p0 + 1.
READ TABLE missing WITH TABLE KEY table_line = idx TRANSPORTING NO FIELDS.
result = xsdbool( sy-subrc <> 0 ).
ENDMETHOD.
METHOD delete.
DATA idx TYPE i.
DATA empty TYPE ` + elem + `.
IF p0 >= 0 AND p0 < lines( items ).
idx = p0 + 1.
INSERT idx INTO TABLE missing.
MODIFY items FROM empty INDEX idx.
ENDIF.
result = abap_true.
ENDMETHOD.
METHOD put.
IF p0 < 0 OR p0 >= 2147483647.
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
DATA idx TYPE i.
DATA empty TYPE ` + elem + `.
DATA hole TYPE i.
idx = p0 + 1.
WHILE lines( items ) < idx.
APPEND empty TO items.
hole = lines( items ).
INSERT hole INTO TABLE missing.
ENDWHILE.
DELETE missing WHERE table_line = idx.
MODIFY items FROM p1 INDEX idx.
ENDMETHOD.
METHOD allocate.
IF p0 < 0.
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
DATA empty TYPE ` + elem + `.
DATA hole TYPE i.
WHILE lines( items ) < p0.
APPEND empty TO items.
hole = lines( items ).
INSERT hole INTO TABLE missing.
ENDWHILE.
ENDMETHOD.
METHOD dense.
IF missing IS NOT INITIAL.
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
ENDMETHOD.
`
}

// Dense operations keep their existing ABI until their sparse variants are
// proven. Refuse before touching storage, never interpret padding as values.
func sparseArrayMethod(name, source string) string {
	switch name {
	case "push", "length":
		return source
	case "get":
		return "IF has( p0 ) = abap_false.\nRETURN.\nENDIF.\n" + source
	case "pop":
		return "DATA hole TYPE i.\nhole = lines( items ).\nIF has( hole - 1 ) = abap_false AND hole > 0.\nDELETE items INDEX hole.\nDELETE missing WHERE table_line = hole.\nRETURN.\nENDIF.\n" + source
	case "reverse":
		return "DATA holes TYPE missing_type.\nDATA hole TYPE i.\nDATA moved TYPE i.\nLOOP AT missing INTO hole.\nmoved = lines( items ) + 1 - hole.\nINSERT moved INTO TABLE holes.\nENDLOOP.\nmissing = holes.\n" + source
	case "concat":
		return "dense( ).\np0->dense( ).\n" + source
	case "slice0":
		return strings.Replace(source, "CREATE OBJECT result.\n", "CREATE OBJECT result.\nresult->missing = missing.\n", 1)
	default:
		return "dense( ).\n" + source
	}
}
