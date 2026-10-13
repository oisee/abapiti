package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestRecordStringDefaultObjectAndRefusals(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	m := method("text", str, ret(rt("dynamic.recordString", local("input", dyn), str)))
	m.Static = true
	m.Params = []hir.Param{{Name: "input", Type: dyn}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "RecordString", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	d, owner := names.Get("runtime.dynamic"), names.Get("RecordString")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_text DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_text IMPLEMENTATION.\nMETHOD check.\n")
	fmt.Fprintf(&unit, "DATA input TYPE REF TO %s.\nDATA text TYPE string.\nDATA refused TYPE abap_bool.\n", d)
	check := func(want string) {
		fmt.Fprintf(&unit, "text = %s=>%s( input ).\ncl_abap_unit_assert=>assert_equals( act = text exp = `%s` ).\n", owner, names.Get("member.text"), want)
	}
	check("undefined")
	fmt.Fprintf(&unit, "CREATE OBJECT input.\ninput->tag = %s=>tag_object.\n", d)
	check("[object Object]")
	unit.WriteString("input->put( p0 = 'data' p1 = VALUE #( ) ).\n")
	check("[object Object]")
	for _, key := range []string{"toString", "valueOf", "__proto__"} {
		fmt.Fprintf(&unit, "input->put( p0 = `%s` p1 = VALUE #( ) ).\nCLEAR refused.\nTRY.\ntext = %s=>%s( input ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nCLEAR input->entries.\n", key, owner, names.Get("member.text"), names.Get("exception.unexecuted"))
	}
	for _, tag := range []string{"tag_array", "tag_ref"} {
		fmt.Fprintf(&unit, "input->tag = %s=>%s.\nCLEAR refused.\nTRY.\ntext = %s=>%s( input ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", d, tag, owner, names.Get("member.text"), names.Get("exception.unexecuted"))
	}
	fmt.Fprintf(&unit, "input->tag = %s=>tag_string.\ninput->sval = `primitive`.\n", d)
	check("primitive")
	fmt.Fprintf(&unit, "input->tag = %s=>tag_null.\n", d)
	check("null")
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
