package abap

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestU3DynamicCollectionBrands(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	m := hir.T(hir.OrderedMap, str, str)
	s := hir.T(hir.OrderedSet, str)
	src := u3Emit(t, rt("dynamic.isMap", rt("dynamic.of", rt("map.markJS", newObj(m), m), dyn), boolean), rt("dynamic.isSet", rt("dynamic.of", newObj(s), dyn), boolean), rt("dynamic.isMap", rt("dynamic.of", newObj(m), dyn), boolean), rt("dynamic.isSet", rt("dynamic.null", lit(0), dyn), boolean))
	u3Want(t, src, "->kind( ) = 1", "->kind( ) = 2", "IF js_map = abap_true.\nresult = 1.", "result = 2.", "CATCH cx_sy_move_cast_error.")
}
func TestU3DynamicValues(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, dyn)
	m := hir.T(hir.OrderedMap, str, str)
	var expressions []*hir.Expr
	for _, v := range []*hir.Expr{hir.L(str, ""), hir.L(str, "😀"), hir.L(hir.T(hir.Number), 1), hir.L(boolean, true), newObj(m), newObj(hir.T(hir.Array, str)), newObj(hir.T(hir.OrderedSet, str))} {
		expressions = append(expressions, rt("dynamic.values", rt("dynamic.of", v, dyn), arr))
	}
	for _, special := range []string{"NaN", "Infinity", "-Infinity"} {
		value := rt("dynamic.of", rt("string.parseFloatNumber", hir.L(str, special), hir.T(hir.Number)), dyn)
		expressions = append(expressions, rt("dynamic.values", value, arr), rt("dynamic.isSet", value, boolean), rt("dynamic.isMap", value, boolean))
	}
	src := u3Emit(t, expressions...)
	u3Want(t, src, "->properties( ).", "=>tag_null.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.", "(1).", "IF js_map = abap_false.", "SORT ordered BY category number position.", "->missing WITH TABLE KEY table_line = graph_slot", "->get(", "WHEN OTHERS.")
}
