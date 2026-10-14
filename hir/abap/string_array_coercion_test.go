package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestNativeStringArrayCoercion(t *testing.T) {
	array := hir.T(hir.Array, str)
	m := method("text", str, ret(rt("dynamic.toString", rt("dynamic.of", local("input", array), hir.T(hir.Dynamic)), str)))
	m.Static = true
	m.Params = []hir.Param{{Name: "input", Type: array}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "StringArrayProbe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	want, err := exec.Command("node", "-e", "process.stdout.write(String(['alpha ', '', 'a,b', 'λ']))").Output()
	if err != nil {
		t.Fatal(err)
	}
	owner, arr := names.Get("StringArrayProbe"), names.Get("runtime."+array.String())
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_text DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_text IMPLEMENTATION.
METHOD check.
DATA a TYPE REF TO %s.
DATA text TYPE string.
DATA refused TYPE abap_bool.
text = %s=>%s( a ).
cl_abap_unit_assert=>assert_equals( act = text exp = 'undefined' ).
CREATE OBJECT a.
text = %s=>%s( a ).
cl_abap_unit_assert=>assert_equals( act = text exp = '' ).
APPEND ~alpha ~ TO a->items.
APPEND ~~ TO a->items.
APPEND ~a,b~ TO a->items.
APPEND ~λ~ TO a->items.
text = %s=>%s( a ).
cl_abap_unit_assert=>assert_equals( act = text exp = %s ).
a->put( p0 = 0 p1 = ~changed~ ).
text = %s=>%s( a ).
cl_abap_unit_assert=>assert_equals( act = text exp = ~changed,,a,b,λ~ ).
a->delete( 1 ).
TRY.
text = %s=>%s( a ).
CATCH cx_sy_range_out_of_bounds.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, arr, owner, names.Get("member.text"), owner, names.Get("member.text"), owner, names.Get("member.text"), "`"+string(want)+"`", owner, names.Get("member.text"), owner, names.Get("member.text"))
	files[owner+".clas.testclasses.abap"] = strings.ReplaceAll(files[owner+".clas.testclasses.abap"], "~", "`")
	storageHostFixture(t, files, owner)
}
