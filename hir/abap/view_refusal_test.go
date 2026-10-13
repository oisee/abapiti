package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestViewRefusalDiagnostics(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	record := hir.Ref("DiagnosticRecord")
	m := method("activate", record, ret(rt("dynamic.classView", local("input", dyn), record)))
	m.Static = true
	m.Params = []hir.Param{{Name: "input", Type: dyn}}
	mapType := hir.T(hir.OrderedMap, str, str)
	box := method("box", dyn, ret(rt("dynamic.of", newObj(mapType), dyn)))
	box.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "DiagnosticRecord", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}, {Name: "Diagnostics", Methods: []*hir.Method{m, box}}}})
	if err != nil {
		t.Fatal(err)
	}
	trap := names.Get("view.refusal")
	for file, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if strings.Contains(line, "RAISE EXCEPTION TYPE "+trap+".") {
				t.Fatalf("%s: view refusal has no reason: %s", file, line)
			}
		}
	}
	owner := names.Get("Diagnostics")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_diagnostics DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_diagnostics IMPLEMENTATION.
METHOD check.
DATA storage TYPE REF TO %s.
DATA native TYPE REF TO %s.
DATA undef TYPE REF TO %s.
DATA refused TYPE abap_bool.
native = NEW #( ).
storage = native.
TRY.
storage->put( p0 = 'key' p1 = undef ).
CATCH %s INTO DATA(error).
refused = abap_true.
cl_abap_unit_assert=>assert_equals( act = error->get_text( ) exp = 'ABAP view refused: live_record_map.go: put: undefined cannot inhabit required map value' ).
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, names.Get("view.storage"), names.Get("runtime."+mapType.String()), names.Get("runtime.dynamic"), trap)
	storageHostFixture(t, files, owner)
}
