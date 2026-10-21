package abap

import (
	"github.com/oisee/abapiti/hir"
	"math"
	"testing"
)

func TestU3ArraySpread(t *testing.T) {
	for _, elem := range []hir.Type{str, hir.T(hir.Optional, str), hir.Ref(hir.RootObject)} {
		arr := hir.T(hir.Array, elem)
		result := arr
		if elem.Kind != hir.Optional && !elem.IsRef() {
			result = hir.T(hir.Array, hir.T(hir.Optional, elem))
		}
		src := u3Emit(t, rt("array.spread0", &hir.Expr{Kind: hir.New, Type: arr, Args: []*hir.Expr{lit(3)}}, result))
		u3Want(t, src, "->missing WITH TABLE KEY table_line = ", "IF sy-subrc <> 0.", "->items.", "CREATE OBJECT ")
	}
}
func TestU3CollectionIdentityQueries(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	for _, key := range []hir.Type{str, hir.T(hir.Number), hir.Ref(hir.RootObject)} {
		m := hir.T(hir.OrderedMap, key, str)
		s := hir.T(hir.OrderedSet, key)
		src := u3Emit(t, rt("collection.has", newObj(m), boolean, rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)), rt("collection.get", newObj(m), hir.T(hir.Optional, str), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)), rt("collection.has", newObj(s), boolean, rt("dynamic.of", hir.L(str, "x"), dyn)))
		u3Want(t, src, "->keys(", "->values(", "->strict_equals(", "->number_special = 1.", "IS NOT BOUND AND ", "EXIT.\nENDIF.\nENDLOOP.")
	}
}
func TestU3SetIterationValues(t *testing.T) {
	s := hir.T(hir.OrderedSet, str)
	src := u3Emit(t, rt("set.iterationValues", newObj(s), hir.T(hir.Array, str)), rt("set.clear", newObj(s), hir.T(hir.Void)))
	u3Want(t, src, "METHOD iterationValues.\nIF iteration IS NOT BOUND.\niteration = values( ).\nENDIF.\nresult = iteration.", "APPEND p0 TO iteration->items.", "ignored = iteration->delete( cursor - 1 ).", "ignored = iteration->delete( cursor ).", "->clear_iteration( ).")
}

func TestU3SetIterationNormalizesNegativeZero(t *testing.T) {
	num := hir.T(hir.Number)
	set := hir.T(hir.OrderedSet, num)
	src := u3Emit(t, rt("set.iterationValues", rt("set.add", newObj(set), set, hir.L(num, math.Copysign(0, -1))), hir.T(hir.Array, num)))
	u3Want(t, src, "IF p0 = 0.\nAPPEND INITIAL LINE TO iteration->items.", "IF <zero> = 0.\n<zero> = 0.")
}

func TestU3SetIterationBoxedReads(t *testing.T) {
	for _, elem := range []hir.Type{hir.T(hir.Optional, hir.T(hir.Number)), hir.T(hir.Optional, str)} {
		set, arr := hir.T(hir.OrderedSet, elem), hir.T(hir.Array, elem)
		m := method("read", arr, ret(rt("set.iterationValues", hir.V("input", set), arr)))
		m.Static = true
		m.Params = []hir.Param{{Name: "input", Type: set}}
		files, err := emitABAP(t, &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}})
		if err != nil {
			t.Fatal(err)
		}
		var src string
		for _, text := range files {
			src += text
		}
		u3Want(t, src, "METHOD iterationValues.", "iteration = values( ).", "->iterationValues(")
	}
}
