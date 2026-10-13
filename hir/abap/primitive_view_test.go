package abap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestPrimitiveBackedConfigView(t *testing.T) {
	dyn, conf := hir.T(hir.Dynamic), hir.Ref("PrimitiveConf")
	optional := hir.T(hir.Optional, hir.T(hir.Number))
	view := rt("dynamic.classView", local("input", dyn), conf)
	field := func(name string, ty hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, X: local("conf", conf), Name: name, Type: ty}
	}
	runMethod := method("run", str, hir.B(decl("conf", conf, view), ret(rt("dynamic.toString", rt("dynamic.of", local("conf", conf), dyn), str))))
	runMethod.Static = true
	runMethod.Params = []hir.Param{{Name: "input", Type: dyn}}
	absent := method("absent", boolean, hir.B(decl("conf", conf, view), ret(binary("==", field("max", optional), hir.L(optional, nil), boolean))))
	absent.Static = true
	absent.Params = runMethod.Params
	required := method("required", hir.T(hir.Number), hir.B(decl("conf", conf, view), ret(field("length", hir.T(hir.Number)))))
	required.Static = true
	required.Params = runMethod.Params
	write := method("write", hir.T(hir.Void), hir.B(decl("conf", conf, view), &hir.Stmt{Kind: hir.Assign, X: field("length", hir.T(hir.Number)), Y: hir.L(hir.T(hir.Number), 1)}))
	write.Static = true
	write.Params = runMethod.Params
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "PrimitiveConf", PrimitiveReads: true, Fields: []hir.Field{{Name: "length", Type: hir.T(hir.Number)}, {Name: "max", Type: optional}}},
		{Name: "Probe", Methods: []*hir.Method{runMethod, absent, required, write}},
	}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner, d := names.Get("Probe"), names.Get("runtime.dynamic")
	unit := fmt.Sprintf(`CLASS ltcl_primitive DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_primitive IMPLEMENTATION.
METHOD check.
DATA input TYPE REF TO %s.
DATA actual TYPE string.
DATA absent TYPE abap_bool.
DATA numeric TYPE f.
DATA refused TYPE abap_bool.
CREATE OBJECT input.
input->tag = %s=>tag_number.
input->nval = 400.
actual = %s=>%s( input ).
cl_abap_unit_assert=>assert_equals( act = actual exp = '400' ).
absent = %s=>%s( input ).
cl_abap_unit_assert=>assert_true( absent ).
TRY.
numeric = %s=>%s( input ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
CLEAR refused.
TRY.
%s=>%s( input ).
CATCH cx_sy_move_cast_error.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, d, d, owner, names.Get("member.run"), owner, names.Get("member.absent"), owner, names.Get("member.required"), names.Get("view.refusal"), owner, names.Get("member.write"))
	// Go's primitive flags admit strings, booleans and Number representations;
	// parser arrays and null retain the structural/prototype refusal boundary.
	tail := ""
	for _, tag := range []string{"tag_array", "tag_null"} {
		tail += fmt.Sprintf("CLEAR refused.\ninput->tag = %s=>%s.\nTRY.\nactual = %s=>%s( input ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", d, tag, owner, names.Get("member.run"), names.Get("view.refusal"))
	}
	unit = strings.Replace(unit, "ENDMETHOD.\nENDCLASS.\n", tail+"ENDMETHOD.\nENDCLASS.\n", 1)
	files[owner+".clas.testclasses.abap"] = unit
	if out := fixtureOutputDir(t); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			source, err = Format(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
