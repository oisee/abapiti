package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

func (b *body) u3StringRuntime(x *hir.Expr, n string) bool {
	switch x.Op {
	case "string.padEnd", "string.relativeAt", "string.codePoints", "string.sha256Hex", "path.basename":
	default:
		return false
	}
	a := b.expr(x.X)
	args := "text = " + a
	if x.Op == "string.padEnd" || x.Op == "string.relativeAt" {
		// ToIntegerOrInfinity is saturated only beyond the observable allocation
		// and indexing domains; NaN becomes zero and infinities stay out of range.
		index := rtIndex(x.Args[0])
		args += " position = " + b.expr(index)
	}
	if x.Op == "string.padEnd" {
		args += " fill = " + b.expr(x.Args[1])
	}
	b.line(n + " = " + b.e.u3StringHelper(x.Op) + "=>run( " + args + " ).")
	return true
}
func rtIndex(x *hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "number.index", X: x, Type: hir.T(hir.I32)}
}

func (e *emitter) u3StringHelper(op string) string {
	id := "runtime.u3." + op
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	params, result, body := "", "string", ""
	switch op {
	case "string.padEnd":
		params = " position TYPE i fill TYPE string"
		body = `DATA needed TYPE i.
DATA take TYPE i.
DATA piece TYPE string.
DATA fragments TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
result = text.
IF position <= strlen( text ) OR fill = $EMPTY.
RETURN.
ENDIF.
IF position > 1048576.
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
APPEND text TO fragments.
needed = position - strlen( text ).
WHILE needed > 0.
take = nmin( val1 = needed val2 = strlen( fill ) ).
piece = fill+0(take).
APPEND piece TO fragments.
needed = needed - take.
ENDWHILE.
CONCATENATE LINES OF fragments INTO result RESPECTING BLANKS.
`
	case "string.relativeAt":
		params = " position TYPE i"
		result = e.typ(hir.T(hir.Optional, hir.T(hir.String)))
		body = `DATA offset TYPE int8.
offset = position.
IF offset < 0.
offset = offset + strlen( text ).
ENDIF.
IF offset < 0 OR offset >= strlen( text ).
RETURN.
ENDIF.
DATA idx TYPE i.
idx = offset.
CREATE OBJECT result.
result->has = abap_true.
result->value = text+idx(1).
`
	case "string.codePoints":
		result = e.typ(hir.T(hir.Array, hir.T(hir.String)))
		body = `DATA cursor TYPE i.
DATA width TYPE i.
DATA code TYPE i.
DATA low TYPE i.
DATA ch TYPE string.
DATA part TYPE string.
CREATE OBJECT result.
WHILE cursor < strlen( text ).
width = 1.
ch = text+cursor(1).
code = unit( ch ).
IF code >= 55296 AND code <= 56319 AND cursor + 1 < strlen( text ).
DATA next TYPE i.
next = cursor + 1.
ch = text+next(1).
low = unit( ch ).
IF low >= 56320 AND low <= 57343.
width = 2.
ENDIF.
ENDIF.
part = text+cursor(width).
APPEND part TO result->items.
cursor = cursor + width.
ENDWHILE.
`
	case "string.sha256Hex":
		// Node's UTF-8 encoder replaces isolated surrogate units with U+FFFD.
		body = `DATA cursor TYPE i.
DATA width TYPE i.
DATA code TYPE i.
DATA low TYPE i.
DATA ch TYPE string.
DATA part TYPE string.
DATA clean TYPE string.
DATA pieces TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
WHILE cursor < strlen( text ).
width = 1.
ch = text+cursor(1).
code = unit( ch ).
part = ch.
IF code >= 55296 AND code <= 56319.
IF cursor + 1 < strlen( text ).
DATA next TYPE i.
next = cursor + 1.
ch = text+next(1).
low = unit( ch ).
IF low >= 56320 AND low <= 57343.
width = 2.
part = text+cursor(2).
ENDIF.
ENDIF.
IF width = 1.
part = cl_abap_conv_in_ce=>uccp( 'FFFD' ).
ENDIF.
ELSEIF code >= 56320 AND code <= 57343.
part = cl_abap_conv_in_ce=>uccp( 'FFFD' ).
ENDIF.
APPEND part TO pieces.
cursor = cursor + width.
ENDWHILE.
CONCATENATE LINES OF pieces INTO clean RESPECTING BLANKS.
DATA converter TYPE REF TO cl_abap_conv_out_ce.
DATA bytes TYPE xstring.
converter = cl_abap_conv_out_ce=>create( encoding = 'UTF-8' ).
converter->convert( EXPORTING data = clean IMPORTING buffer = bytes ).
cl_abap_message_digest=>calculate_hash_for_raw( EXPORTING if_algorithm = 'SHA256' if_data = bytes IMPORTING ef_hashstring = result ).
TRANSLATE result TO LOWER CASE.
`
	case "path.basename":
		// The HIR operation has no ext parameter (nodePlatformSignature).
		body = `DATA start TYPE i.
DATA stop TYPE i.
DATA cursor TYPE i.
DATA count TYPE i.
stop = strlen( text ).
WHILE stop > 0.
cursor = stop - 1.
IF text+cursor(1) <> '/'.
EXIT.
ENDIF.
stop = cursor.
ENDWHILE.
cursor = stop.
WHILE cursor > 0.
cursor = cursor - 1.
IF text+cursor(1) = '/'.
start = cursor + 1.
EXIT.
ENDIF.
ENDWHILE.
count = stop - start.
result = substring( val = text off = start len = count ).
`
	}
	unitDef, unitImpl := "", ""
	if op == "string.codePoints" || op == "string.sha256Hex" {
		unitDef = "CLASS-METHODS unit IMPORTING ch TYPE string RETURNING VALUE(result) TYPE i.\n"
		unitImpl = `METHOD unit.
DATA converter TYPE REF TO cl_abap_conv_out_ce.
DATA bytes TYPE xstring.
DATA low TYPE x LENGTH 1.
DATA high TYPE x LENGTH 1.
DATA hi TYPE i.
converter = cl_abap_conv_out_ce=>create( encoding = '4103' ).
converter->convert( EXPORTING data = ch IMPORTING buffer = bytes ).
low = bytes+0(1).
high = bytes+1(1).
result = low.
hi = high.
result = result + hi * 256.
ENDMETHOD.
`
	}
	code := "CLASS " + name + " DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS run IMPORTING text TYPE string" + params + " RETURNING VALUE(result) TYPE " + result + ".\nPROTECTED SECTION.\nPRIVATE SECTION.\n" + unitDef + "ENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nMETHOD run.\n" + body + "ENDMETHOD.\n" + unitImpl + "ENDCLASS.\n"
	e.files[name+".clas.abap"] = strings.ReplaceAll(code, "$EMPTY", "``")
	return name
}
