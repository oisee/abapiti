package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestChaiNativeNumberZeroSigns(t *testing.T) {
	d, number := hir.T(hir.Dynamic), hir.T(hir.Number)
	arr := hir.T(hir.Array, number)
	box := method("box", d, ret(rt("chai.box", local("value", number), d)))
	box.Static, box.Params = true, []hir.Param{{Name: "value", Type: number}}
	array := method("array", d, ret(rt("chai.box", local("value", arr), d)))
	array.Static, array.Params = true, []hir.Param{{Name: "value", Type: arr}}
	equal := method("equal", boolean, ret(rt("chai.deepEqual", local("a", d), boolean, local("b", d))))
	equal.Static, equal.Params = true, []hir.Param{{Name: "a", Type: d}, {Name: "b", Type: d}}
	negate := method("negate", number, ret(&hir.Expr{Kind: hir.Unary, Op: "-", Type: number, X: local("value", number)}))
	negate.Static, negate.Params = true, box.Params
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "NativeZeroProbe", Methods: []*hir.Method{box, array, equal, negate}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("NativeZeroProbe")
	if !strings.Contains(files[names.Get("runtime.chai")+".clas.abap"], "DECIMALS = 0") {
		t.Fatal("native numeric array adapter must observe signed zero")
	}
	var unit strings.Builder
	fmt.Fprintf(&unit, "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA positive TYPE REF TO %s.\nDATA negative TYPE REF TO %s.\nDATA a TYPE REF TO %s.\nDATA b TYPE REF TO %s.\nDATA x TYPE REF TO %s.\nDATA y TYPE REF TO %s.\npositive = %s=>%s( CONV f( '0' ) ).\nnegative = %s=>%s( %s=>%s( CONV f( '0' ) ) ).\n", names.Get("runtime.dynamic"), names.Get("runtime.dynamic"), names.Get("runtime."+arr.String()), names.Get("runtime."+arr.String()), names.Get("runtime.dynamic"), names.Get("runtime.dynamic"), owner, names.Get("member.box"), owner, names.Get("member.box"), owner, names.Get("member.negate"))
	compare := func(a, b string) string {
		return owner + "=>" + names.Get("member.equal") + "( " + names.Get("param.a") + " = " + a + " " + names.Get("param.b") + " = " + b + " )"
	}
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_true( %s ).\ncl_abap_unit_assert=>assert_true( %s ).\ncl_abap_unit_assert=>assert_false( %s ).\na = NEW #( ).\nb = NEW #( ).\nAPPEND CONV f( '0' ) TO a->items.\nAPPEND CONV f( '-0' ) TO b->items.\nx = %s=>%s( a ).\ny = %s=>%s( b ).\ncl_abap_unit_assert=>assert_false( %s ).\nb->items = a->items.\ny = %s=>%s( b ).\ncl_abap_unit_assert=>assert_true( %s ).\nENDMETHOD.\nENDCLASS.\n", compare("positive", "positive"), compare("negative", "negative"), compare("positive", "negative"), owner, names.Get("member.array"), owner, names.Get("member.array"), compare("x", "y"), owner, names.Get("member.array"), compare("x", "y"))
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
