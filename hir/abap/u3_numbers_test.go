package abap

import (
	"github.com/oisee/abapiti/hir"
	"math"
	"strings"
	"testing"
)

func u3Emit(t *testing.T, expressions ...*hir.Expr) string {
	t.Helper()
	var methods []*hir.Method
	for i, x := range expressions {
		m := method("probe"+string(rune('a'+i)), x.Type, ret(x))
		m.Static = true
		methods = append(methods, m)
	}
	files, err := emitABAP(t, &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: methods}}})
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Errorf("ABAP line exceeds 255: %s", line)
			}
			parts = append(parts, strings.TrimSpace(line))
		}
	}
	return strings.Join(parts, "\n")
}
func u3Want(t *testing.T, src string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(src, snippet) {
			t.Errorf("missing ABAP snippet %q", snippet)
		}
	}
}
func TestU3NumberPredicates(t *testing.T) {
	num, dyn := hir.T(hir.Number), hir.T(hir.Dynamic)
	for _, op := range []string{"number.isInteger", "number.isSafeInteger", "number.isFinite"} {
		t.Run(op, func(t *testing.T) {
			var expressions []*hir.Expr
			for _, v := range []float64{0, math.Copysign(0, -1), 1.5, 9007199254740991, 9007199254740992, -9007199254740992} {
				expressions = append(expressions, rt(op, rt("dynamic.of", hir.L(num, v), dyn), hir.T(hir.Bool)))
			}
			for _, v := range []string{"NaN", "Infinity", "-Infinity"} {
				expressions = append(expressions, rt(op, rt("dynamic.of", rt("string.parseFloatNumber", hir.L(str, v), num), dyn), hir.T(hir.Bool)))
			}
			expressions = append(expressions, rt(op, rt("dynamic.of", hir.L(str, "1"), dyn), hir.T(hir.Bool)))
			src := u3Emit(t, expressions...)
			u3Want(t, src, "->number_special = 0.", "=>tag_number.")
			if op != "number.isFinite" {
				u3Want(t, src, "->nval = trunc(")
			}
			if op == "number.isSafeInteger" {
				u3Want(t, src, "<= 9007199254740991")
			}
		})
	}
}
func TestU3NumberAbsAndHex(t *testing.T) {
	num := hir.T(hir.Number)
	var expressions []*hir.Expr
	for _, text := range []string{"NaN", "Infinity", "-Infinity", "-0", "-1.5"} {
		expressions = append(expressions, rt("number.abs", rt("string.parseFloatNumber", hir.L(str, text), num), num))
	}
	for _, value := range []float64{math.Copysign(0, -1), 0, 15, -15, 9007199254740991, -9007199254740991, 9007199254740992, 1.5} {
		expressions = append(expressions, rt("number.hex16", hir.L(num, value), str))
	}
	src := u3Emit(t, expressions...)
	u3Want(t, src, "->number_special = 3.", "->number_special = 2.", "->value = abs(", "magnitude MOD 16.", "magnitude DIV 16.", "IF value = 0.", "abs( value ) > 9007199254740991.", "result = '-' && result.")
}
