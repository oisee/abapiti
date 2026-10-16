package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

func init() {
	for _, op := range []string{"bigint.decimal", "bigint.neg", "bigint.toString", "bigint.compare", "dynamic.asBigInt"} {
		supportedOps[op] = true
	}
}

func (b *body) bigintRuntime(x *hir.Expr, n string) bool {
	if !strings.HasPrefix(x.Op, "bigint.") && x.Op != "dynamic.asBigInt" {
		return false
	}
	a := b.expr(x.X)
	if x.Op == "dynamic.asBigInt" {
		b.line("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.\nIF " + a + "->tag <> " + b.e.name("runtime.dynamic") + "=>tag_bigint.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.\n" + n + " = " + a + "->sval.")
		return true
	}
	rt := b.e.bigintRuntime()
	switch x.Op {
	case "bigint.decimal":
		loc := b.temp(hir.T(hir.String))
		b.stringLit(loc, x.Source)
		b.line(n + " = " + rt + "=>decimal( p0 = " + a + " origin = " + loc + " ).")
	case "bigint.toString":
		b.line(n + " = " + a + ".")
	case "bigint.neg":
		b.line(n + " = " + rt + "=>neg( " + a + " ).")
	case "bigint.compare":
		b.line(n + " = " + rt + "=>compare( p0 = " + a + " p1 = " + b.expr(x.Args[0]) + " ).")
	}
	return true
}
func (e *emitter) bigintRuntime() string {
	id := "runtime.bigint"
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	refusal := e.name("exception.BigIntRefusal")
	e.files[refusal+".clas.abap"] = "CLASS " + refusal + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA reason TYPE string.\nMETHODS constructor IMPORTING reason TYPE string.\nMETHODS get_text REDEFINITION.\nENDCLASS.\nCLASS " + refusal + " IMPLEMENTATION.\nMETHOD constructor.\nsuper->constructor( ).\nme->reason = reason.\nENDMETHOD.\nMETHOD get_text.\nresult = reason.\nENDMETHOD.\nENDCLASS.\n"
	code := `CLASS $P DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS decimal IMPORTING p0 TYPE string origin TYPE string RETURNING VALUE(result) TYPE string.
CLASS-METHODS neg IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE string.
CLASS-METHODS compare IMPORTING p0 TYPE string p1 TYPE string RETURNING VALUE(result) TYPE f.
PRIVATE SECTION.
CLASS-METHODS whitespace IMPORTING ch TYPE string RETURNING VALUE(result) TYPE abap_bool.
ENDCLASS.
CLASS $P IMPLEMENTATION.
METHOD whitespace.
DATA cp TYPE i.
DATA converter TYPE REF TO cl_abap_conv_out_ce.
DATA bytes TYPE xstring.
DATA low TYPE x LENGTH 1.
DATA high TYPE x LENGTH 1.
DATA hi TYPE i.
cp = cl_abap_conv_out_ce=>uccpi( ch ).
IF cp >= 255.
 converter = cl_abap_conv_out_ce=>create( encoding = '4103' ).
 converter->convert( EXPORTING data = ch IMPORTING buffer = bytes ).
 low = bytes+0(1).
 high = bytes+1(1).
 cp = low.
 hi = high.
 cp = cp + hi * 256.
ENDIF.
result = xsdbool( cp = 9 OR cp = 10 OR cp = 11 OR cp = 12 OR cp = 13 OR cp = 32 OR cp = 160 OR cp = 5760 OR ( cp >= 8192 AND cp <= 8202 ) OR cp = 8232 OR cp = 8233 OR cp = 8239 OR cp = 8287 OR cp = 12288 OR cp = 65279 ).
ENDMETHOD.
METHOD decimal.
DATA text TYPE string.
DATA negative TYPE abap_bool.
DATA i TYPE i.
DATA n TYPE i.
DATA c TYPE string.
text = p0.
WHILE strlen( text ) > 0.
 IF whitespace( text(1) ) = abap_false.
  EXIT.
 ENDIF.
 text = text+1.
ENDWHILE.
WHILE strlen( text ) > 0.
 n = strlen( text ) - 1.
 IF whitespace( text+n(1) ) = abap_false.
  EXIT.
 ENDIF.
 text = text(n).
ENDWHILE.
IF strlen( text ) > 0 AND ( text(1) = '-' OR text(1) = '+' ).
 negative = xsdbool( text(1) = '-' ).
 text = text+1.
 IF text IS INITIAL.
  RAISE EXCEPTION TYPE $E EXPORTING reason = origin && ': BigInt requires a decimal integer string'.
 ENDIF.
ENDIF.
WHILE i < strlen( text ).
 c = text+i(1).
 IF c CN '0123456789'.
  RAISE EXCEPTION TYPE $E EXPORTING reason = origin && ': BigInt supports decimal integer strings only'.
 ENDIF.
 i = i + 1.
ENDWHILE.
WHILE strlen( text ) > 0 AND text(1) = '0'.
 text = text+1.
ENDWHILE.
IF text IS INITIAL.
 result = '0'.
ELSEIF negative = abap_true.
 result = '-' && text.
ELSE.
 result = text.
ENDIF.
ENDMETHOD.
METHOD neg.
IF p0 = '0'.
 result = p0.
ELSEIF p0(1) = '-'.
 result = p0+1.
ELSE.
 result = '-' && p0.
ENDIF.
ENDMETHOD.
METHOD compare.
DATA a TYPE string.
DATA b TYPE string.
DATA an TYPE abap_bool.
DATA bn TYPE abap_bool.
DATA sign TYPE i VALUE 1.
a = p0.
b = p1.
an = xsdbool( a(1) = '-' ).
bn = xsdbool( b(1) = '-' ).
IF an <> bn.
 IF an = abap_true.
  result = -1.
 ELSE.
  result = 1.
 ENDIF.
 RETURN.
ENDIF.
IF an = abap_true.
 a = a+1.
 b = b+1.
 sign = -1.
ENDIF.
IF strlen( a ) < strlen( b ) OR ( strlen( a ) = strlen( b ) AND a < b ).
 result = 0 - sign.
ELSEIF a = b.
 result = 0.
ELSE.
 result = sign.
ENDIF.
ENDMETHOD.
ENDCLASS.
`
	e.files[name+".clas.abap"] = strings.NewReplacer("$P", name, "$E", refusal).Replace(code)
	return name
}
