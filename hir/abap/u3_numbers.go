package abap

import "github.com/oisee/abapiti/hir"

func (b *body) u3NumberRuntime(x *hir.Expr, n string) bool {
	switch x.Op {
	case "number.isInteger", "number.isSafeInteger", "number.isFinite":
		a := b.expr(x.X)
		d := b.e.name("runtime.dynamic")
		b.line("CLEAR " + n + ".")
		b.line("IF " + a + " IS BOUND AND " + a + "->tag = " + d + "=>tag_number.")
		b.line("IF " + a + "->number_special = 0.")
		test := "abap_true"
		if x.Op != "number.isFinite" {
			test = "xsdbool( " + a + "->nval = trunc( " + a + "->nval )"
			if x.Op == "number.isSafeInteger" {
				test += " AND abs( " + a + "->nval ) <= 9007199254740991"
			}
			test += " )"
		}
		b.line(n + " = " + test + ".")
		b.line("ENDIF.\nENDIF.")
	case "number.abs":
		a := b.expr(x.X)
		if taggedNumber(x.X.Type) {
			b.line("CREATE OBJECT " + n + ".")
			b.line(n + "->has = abap_true.")
			b.line("IF " + a + " IS BOUND AND " + a + "->has = abap_true.")
			b.line(n + "->number_special = " + a + "->number_special.")
			b.line("IF " + n + "->number_special = 3.\n" + n + "->number_special = 2.\nENDIF.")
			b.line(n + "->value = abs( " + a + "->value ).")
			b.line("ELSE.\n" + n + "->number_special = 1.\nENDIF.")
		} else {
			b.line(n + " = abs( " + a + " ).")
		}
	case "number.hex16":
		a := b.scalarNumber(x.X)
		b.line(n + " = " + b.e.hex16Runtime() + "=>format( " + a + " ).")
	default:
		return false
	}
	return true
}

func (e *emitter) hex16Runtime() string {
	id := "runtime.hex16"
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	e.files[name+".clas.abap"] = "CLASS " + name + ` DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS format IMPORTING value TYPE f RETURNING VALUE(result) TYPE string.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS ` + name + ` IMPLEMENTATION.
METHOD format.
DATA magnitude TYPE int8.
DATA digit TYPE i.
DATA negative TYPE abap_bool.
DATA alphabet TYPE string VALUE '0123456789abcdef'.
IF value <> trunc( value ) OR abs( value ) > 9007199254740991.
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
IF value = 0.
result = '0'.
RETURN.
ENDIF.
negative = xsdbool( value < 0 ).
magnitude = abs( value ).
WHILE magnitude > 0.
digit = magnitude MOD 16.
result = alphabet+digit(1) && result.
magnitude = magnitude DIV 16.
ENDWHILE.
IF negative = abap_true.
result = '-' && result.
ENDIF.
ENDMETHOD.
ENDCLASS.
`
	return name
}
