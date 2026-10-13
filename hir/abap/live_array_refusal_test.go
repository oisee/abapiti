package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLiveArrayRefusalFixtures(t *testing.T) {
	dyn, arr := hir.T(hir.Dynamic), hir.T(hir.Array, str)
	read := method("read", str, hir.B(decl("graph", dyn, rt("json.parseSubset", local("input", str), dyn)), decl("a", arr, rt("dynamic.arrayView", local("graph", dyn), arr)), ret(&hir.Expr{Kind: hir.IndexGet, Type: str, X: local("a", arr), Y: lit(0)})))
	read.Static = true
	read.Params = []hir.Param{{Name: "input", Type: str}}
	poison := method("poison", str, hir.B(decl("graph", dyn, rt("json.parseSubset", hir.L(str, `["valid"]`), dyn)), decl("a", arr, rt("dynamic.arrayView", local("graph", dyn), arr)), run(rt("dynamic.put", local("graph", dyn), hir.T(hir.Void), hir.L(str, "0"), rt("dynamic.of", lit(7), dyn))), ret(&hir.Expr{Kind: hir.IndexGet, Type: str, X: local("a", arr), Y: lit(0)})))
	poison.Static = true
	files, names, err := EmitNamed(&hir.Program{Classes: []*hir.Class{{Name: "ArrayBoundaries", Methods: []*hir.Method{read, poison}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("ArrayBoundaries")
	var unit strings.Builder
	fmt.Fprint(&unit, "CLASS ltcl_bounds DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_bounds IMPLEMENTATION.\nMETHOD check.\nDATA result TYPE string.\nDATA refused TYPE abap_bool.\n")
	fmt.Fprintf(&unit, "result = %s=>%s( `[\"valid\"]` ).\ncl_abap_unit_assert=>assert_equals( act = result exp = `valid` ).\n", owner, names.Get("member.read"))
	for _, input := range []string{`[]`, `[null]`, `[1]`, `{}`} {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nresult = %s=>%s( `%s` ).\nCATCH cx_sy_range_out_of_bounds cx_sy_move_cast_error %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member.read"), input, names.Get("view.refusal"))
	}
	fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nresult = %s=>%s( ).\nCATCH cx_sy_move_cast_error.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.poison"))
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
