package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func taggedNumber(t hir.Type) bool { return t.Equal(hir.T(hir.Optional, hir.T(hir.Number))) }

func (b *body) numberStorageExpr(x *hir.Expr, n string) bool {
	if x.Kind == hir.Narrow && taggedNumber(x.Type) && taggedNumber(x.X.Type) {
		b.line(n + " = " + b.expr(x.X) + ".")
		return true
	}
	if x.Kind == hir.RuntimeOp && x.Op == "string.parseFloatNumber" && taggedNumber(x.Type) {
		b.line(n + " = " + b.e.stringOpsRuntime() + "=>float_optional( " + b.expr(x.X) + " ).")
		return true
	}
	if x.Kind == hir.Binary && (b.e.numberExprs[x.X] || b.e.numberExprs[x.Y]) {
		switch x.Op {
		case "+", "-", "*", "/", "%", "<", ">", "<=", ">=", "==", "!=":
			a, z := b.value(x.X, hir.T(hir.Optional, hir.T(hir.Number))), b.value(x.Y, hir.T(hir.Optional, hir.T(hir.Number)))
			runtime := b.e.numberBoxRuntime()
			method := "calculate"
			if x.Type.Kind == hir.Bool {
				method = "compare"
			}
			b.line(n + " = " + runtime + "=>" + method + "( left = " + a + " right = " + z + " op = `" + x.Op + "` ).")
			return true
		}
	}
	if x.Kind == hir.NumericMinMax && taggedNumber(x.Type) {
		if x.Y == nil {
			b.e.refuse(fmt.Errorf("node %d: tagged Number array min/max requires tagged array storage", x.ID))
			return true
		}
		t := hir.T(hir.Optional, hir.T(hir.Number))
		a, z := b.value(x.X, t), b.value(x.Y, t)
		b.line(n + " = " + b.e.numberBoxRuntime() + "=>minmax( left = " + a + " right = " + z + " op = `" + x.Op + "` ).")
		return true
	}
	if x.Kind == hir.Unary && taggedNumber(x.Type) && x.Op == "-" {
		b.line(n + " = " + b.e.numberBoxRuntime() + "=>negate( " + b.expr(x.X) + " ).")
		return true
	}
	return false
}

// Runtime operations whose numeric contract is scalar still check a tagged
// argument before unboxing. The index operation implements ToIntegerOrInfinity,
// while stringification/isNaN can observe the special lane directly.
func (b *body) numberStorageRuntime(x *hir.Expr, n string) bool {
	if x.X == nil || !taggedNumber(x.X.Type) {
		return false
	}
	switch x.Op {
	case "number.index":
		b.line(n + " = " + b.e.numberBoxRuntime() + "=>index( " + b.expr(x.X) + " ).")
	case "number.isNaN":
		a := b.expr(x.X)
		b.line(n + " = xsdbool( " + a + "->number_special = 1 ).")
	case "number.toString":
		b.line(n + " = " + b.e.numberBoxRuntime() + "=>text( " + b.expr(x.X) + " ).")
	default:
		if !strings.HasPrefix(x.Op, "number.") {
			return false
		}
		value := b.scalarNumber(x.X)
		key := "\x00number." + value
		b.locals[key] = value
		copy := *x
		copy.X = hir.V(key, hir.T(hir.Number))
		copy.Type = x.Type
		if taggedNumber(x.Type) {
			copy.Type = hir.T(hir.Number)
			finite := b.temp(copy.Type)
			b.runtimeOp(&copy, finite)
			b.line(n + " = " + b.convert(finite, copy.Type, x.Type) + ".")
		} else {
			b.runtimeOp(&copy, n)
		}
		delete(b.locals, key)
	}
	return true
}

func (e *emitter) numberBoxRuntime() string {
	id := "runtime.numberBox"
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	typ := e.typ(hir.T(hir.Optional, hir.T(hir.Number)))
	// Reuse the string-domain exception for genuinely unsupported conversions;
	// producing or carrying NaN itself is no longer a refusal.
	e.stringOpsRuntime()
	code := `CLASS $P DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS calculate IMPORTING left TYPE $N right TYPE $N op TYPE string RETURNING VALUE(result) TYPE $N.
CLASS-METHODS compare IMPORTING left TYPE $N right TYPE $N op TYPE string RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS minmax IMPORTING left TYPE $N right TYPE $N op TYPE string RETURNING VALUE(result) TYPE $N.
CLASS-METHODS negate IMPORTING value TYPE $N RETURNING VALUE(result) TYPE $N.
CLASS-METHODS finite IMPORTING value TYPE $N RETURNING VALUE(result) TYPE f.
CLASS-METHODS index IMPORTING value TYPE $N RETURNING VALUE(result) TYPE i.
CLASS-METHODS text IMPORTING value TYPE $N RETURNING VALUE(result) TYPE string.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS $P IMPLEMENTATION.
METHOD finite.
IF value IS NOT BOUND OR value->has = abap_false OR value->number_special <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
result = value->value.
ENDMETHOD.
METHOD calculate.
DATA a TYPE f.
DATA b TYPE f.
CREATE OBJECT result.
result->has = abap_true.
IF left IS NOT BOUND OR right IS NOT BOUND.
result->number_special = 1.
RETURN.
ENDIF.
IF left->has = abap_false OR right->has = abap_false OR left->number_special = 1 OR right->number_special = 1.
result->number_special = 1.
RETURN.
ENDIF.
IF left->number_special <> 0 OR right->number_special <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
a = left->value.
b = right->value.
CASE op.
WHEN '+'.
result->value = a + b.
WHEN '-'.
result->value = a - b.
WHEN '*'.
result->value = a * b.
WHEN '/'.
IF b = 0.
IF a = 0.
result->number_special = 1.
ELSE.
RAISE EXCEPTION TYPE $E.
ENDIF.
ELSE.
result->value = a / b.
ENDIF.
WHEN '%'.
IF b = 0.
result->number_special = 1.
ELSE.
RAISE EXCEPTION TYPE $E.
ENDIF.
WHEN OTHERS.
RAISE EXCEPTION TYPE $E.
ENDCASE.
ENDMETHOD.
METHOD compare.
DATA a TYPE f.
DATA b TYPE f.
IF left IS NOT BOUND OR right IS NOT BOUND.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF left->has = abap_false OR right->has = abap_false.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF left->number_special = 1 OR right->number_special = 1.
result = xsdbool( op = '!=' ).
RETURN.
ENDIF.
IF left->number_special <> 0 OR right->number_special <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
a = left->value.
b = right->value.
CASE op.
WHEN '=='.
result = xsdbool( a = b ).
WHEN '!='.
result = xsdbool( a <> b ).
WHEN '<'.
result = xsdbool( a < b ).
WHEN '>'.
result = xsdbool( a > b ).
WHEN '<='.
result = xsdbool( a <= b ).
WHEN '>='.
result = xsdbool( a >= b ).
WHEN OTHERS.
RAISE EXCEPTION TYPE $E.
ENDCASE.
ENDMETHOD.
METHOD minmax.
IF left IS NOT BOUND OR right IS NOT BOUND.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF left->number_special = 1.
result = left.
ELSEIF right->number_special = 1.
result = right.
ELSE.
IF left->number_special <> 0 OR right->number_special <> 0 OR left->value = 0 AND right->value = 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
result = left.
IF op = 'min' AND right->value < left->value OR op = 'max' AND right->value > left->value.
result = right.
ENDIF.
ENDIF.
ENDMETHOD.
METHOD negate.
CREATE OBJECT result.
result->has = abap_true.
IF value IS NOT BOUND.
result->number_special = 1.
RETURN.
ENDIF.
IF value->has = abap_false.
result->number_special = 1.
RETURN.
ENDIF.
result->number_special = value->number_special.
CASE value->number_special.
WHEN 0.
result->value = - value->value.
WHEN 2.
result->number_special = 3.
WHEN 3.
result->number_special = 2.
ENDCASE.
ENDMETHOD.
METHOD index.
IF value IS NOT BOUND OR value->has = abap_false OR value->number_special = 1.
RETURN.
ENDIF.
IF value->number_special = 2 OR value->value > 2147483647.
result = 2147483647.
ELSEIF value->number_special = 3 OR value->value < -2147483648.
result = -2147483648.
ELSE.
result = trunc( value->value ).
ENDIF.
ENDMETHOD.
METHOD text.
IF value IS NOT BOUND OR value->has = abap_false.
result = 'undefined'.
RETURN.
ENDIF.
CASE value->number_special.
WHEN 1.
result = 'NaN'.
WHEN 2.
result = 'Infinity'.
WHEN 3.
result = '-Infinity'.
WHEN OTHERS.
IF abs( value->value ) > 9007199254740991 OR value->value <> trunc( value->value ).
RAISE EXCEPTION TYPE $E.
ENDIF.
DATA integral TYPE int8.
integral = value->value.
result = |{ integral }|.
ENDCASE.
ENDMETHOD.
ENDCLASS.
`
	e.files[name+".clas.abap"] = strings.NewReplacer("$P", name, "$N", typ, "$E", e.name("exception.StringOpRefusal")).Replace(code)
	return name
}

func (b *body) scalarNumber(x *hir.Expr) string {
	v := b.expr(x)
	if taggedNumber(x.Type) {
		result := b.temp(hir.T(hir.Number))
		b.line(fmt.Sprintf("%s = %s=>finite( %s ).", result, b.e.numberBoxRuntime(), v))
		return result
	}
	return v
}
