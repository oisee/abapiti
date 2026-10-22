package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestU3PadEnd(t *testing.T) {
	num := hir.T(hir.Number)
	var expressions []*hir.Expr
	for _, c := range []struct {
		text string
		n    float64
		fill string
	}{{"", 0, ""}, {"x", 6, "ab"}, {"😀", 5, "😀"}, {"abc", 2, " "}, {"x", 4, ""}, {"x", 3.9, "ab"}, {"x", -1, "ab"}} {
		expressions = append(expressions, rt("string.padEnd", hir.L(str, c.text), str, hir.L(num, c.n), hir.L(str, c.fill)))
	}
	for _, special := range []string{"NaN", "Infinity", "-Infinity"} {
		expressions = append(expressions, rt("string.padEnd", hir.L(str, "x"), str, rt("string.parseFloatNumber", hir.L(str, special), num), hir.L(str, "ab")))
	}
	src := u3Emit(t, expressions...)
	u3Want(t, src, "IF position <= strlen( text ) OR fill = ``.", "IF position > 1048576.", "piece = fill+0(take).", "APPEND text TO fragments.", "CONCATENATE LINES OF fragments INTO result RESPECTING BLANKS.", "number_special = 1.")
}
func TestU3RelativeAt(t *testing.T) {
	num := hir.T(hir.Number)
	opt := hir.T(hir.Optional, str)
	var expressions []*hir.Expr
	for _, text := range []string{"", "abc", "😀"} {
		for _, index := range []float64{-4, -1, -0.0, 0, 1, 2, 3, 1.9} {
			expressions = append(expressions, rt("string.relativeAt", hir.L(str, text), opt, hir.L(num, index)))
		}
	}
	for _, special := range []string{"NaN", "Infinity", "-Infinity"} {
		expressions = append(expressions, rt("string.relativeAt", hir.L(str, "😀"), opt, rt("string.parseFloatNumber", hir.L(str, special), num)))
	}
	src := u3Emit(t, expressions...)
	u3Want(t, src, "offset = offset + strlen( text ).", "IF offset < 0 OR offset >= strlen( text ).\nRETURN.", "result->has = abap_true.", "result->value = text+idx(1).", "number_special = 1.")
}
func TestU3CodePoints(t *testing.T) {
	arr := hir.T(hir.Array, str)
	high := rt("string.substring", hir.L(str, "😀"), str, lit(0), lit(1))
	low := rt("string.substring", hir.L(str, "😀"), str, lit(1), lit(2))
	src := u3Emit(t, rt("string.codePoints", hir.L(str, "a😀b"), arr), rt("string.codePoints", hir.L(str, ""), arr), rt("string.codePoints", high, arr), rt("string.codePoints", low, arr))
	u3Want(t, src, "CREATE OBJECT result.\nWHILE cursor < strlen( text ).", "code >= 55296 AND code <= 56319", "low >= 56320 AND low <= 57343.", "width = 2.", "part = text+cursor(width).", "result = result + hi * 256.")
}
func TestU3SHA256(t *testing.T) {
	high := rt("string.substring", hir.L(str, "😀"), str, lit(0), lit(1))
	low := rt("string.substring", hir.L(str, "😀"), str, lit(1), lit(2))
	src := u3Emit(t, rt("string.sha256Hex", hir.L(str, ""), str), rt("string.sha256Hex", hir.L(str, "é😀"), str), rt("string.sha256Hex", high, str), rt("string.sha256Hex", low, str))
	u3Want(t, src, "cl_abap_conv_in_ce=>uccp( 'FFFD' ).", "part = text+cursor(2).", "calculate_hash_for_char( EXPORTING if_algorithm = 'SHA256' if_data = clean IMPORTING ef_hashstring = result", "TRANSLATE result TO LOWER CASE.")
}
func TestU3Basename(t *testing.T) {
	var expressions []*hir.Expr
	for _, path := range []string{"", "/", "///", "/a/b///", "a", "a/", ".foo", "foo.txt", "a\\b"} {
		expressions = append(expressions, rt("path.basename", hir.L(str, path), str))
	}
	src := u3Emit(t, expressions...)
	u3Want(t, src, "stop = strlen( text ).", "IF text+cursor(1) <> '/'.", "stop = cursor.", "start = cursor + 1.", "count = stop - start.", "substring( val = text off = start len = count ).")
}
func TestU3StringHelpersGated(t *testing.T) {
	src := u3Emit(t, hir.L(str, "hello"))
	for _, absent := range []string{"METHOD unit.", "if_algorithm = 'SHA256'", "position TYPE i fill TYPE string", "offset = position.", "stop = strlen( text )."} {
		if strings.Contains(src, absent) {
			t.Errorf("unused helper emitted: %s", absent)
		}
	}
}
