package abap

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestLiveArrayWideWriteRefusesBeforeMutation(t *testing.T) {
	dyn, wide := hir.T(hir.Dynamic), hir.T(hir.Array, hir.T(hir.I64))
	holder := hir.Ref("WideArrayHolder")
	m := method("activate", holder, ret(rt("dynamic.classView", local("input", dyn), holder)))
	m.Static = true
	m.Params = []hir.Param{{Name: "input", Type: dyn}}
	files, names, err := EmitNamed(&hir.Program{Classes: []*hir.Class{{Name: "WideArrayHolder", Structural: true, Fields: []hir.Field{{Name: "a", Type: wide}}}, {Name: "WideArrayBoundary", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner, d, arr := names.Get("WideArrayBoundary"), names.Get("runtime.dynamic"), names.Get("runtime."+wide.String())
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_wide DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_wide IMPLEMENTATION.
METHOD check.
DATA graph TYPE REF TO %s.
DATA value TYPE REF TO %s.
DATA a TYPE REF TO %s.
DATA refused TYPE abap_bool.
CREATE OBJECT graph.
graph->tag = %s=>tag_array.
CREATE OBJECT value.
value->tag = %s=>tag_number.
value->nval = 1.
graph->append( value ).
CREATE OBJECT a.
a->source = graph.
TRY.
a->put( p0 = 0 p1 = 4 ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
cl_abap_unit_assert=>assert_equals( act = lines( graph->items ) exp = 1 ).
READ TABLE graph->items INDEX 1 INTO value.
cl_abap_unit_assert=>assert_equals( act = value->nval exp = 1 ).
CLEAR a->source.
a->put( p0 = 0 p1 = 22 ).
DATA actual TYPE int8.
READ TABLE a->items INDEX 1 INTO actual.
cl_abap_unit_assert=>assert_equals( act = actual exp = 22 ).
ENDMETHOD.
ENDCLASS.
`, d, d, arr, d, d, names.Get("view.refusal"))
	storageHostFixture(t, files, owner)
}
