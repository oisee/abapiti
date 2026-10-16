package abap

import (
	"fmt"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Own-key adapters synthesize an array setter even when the program contains
// no explicit array view. Writes through that setter must retain graph storage.
func TestOwnArraySlotKeepsLiveStorage(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	keys := method("keys", hir.T(hir.Array, str), hir.B(ret(rt("dynamic.keys", local("input", dyn), hir.T(hir.Array, str)))))
	keys.Static = true
	keys.Params = []hir.Param{{Name: "input", Type: dyn}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{
		{Name: "ArraySlot", Structural: true, Fields: []hir.Field{{Name: "items", Type: hir.T(hir.Array, dyn)}}},
		{Name: "ArraySlotProbe", Methods: []*hir.Method{keys}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("ArraySlot")
	d := names.Get("runtime.dynamic")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_array_slot DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_array_slot IMPLEMENTATION.
METHOD check.
DATA shape TYPE REF TO %s.
DATA input TYPE REF TO %s.
DATA graph TYPE REF TO %s.
DATA value TYPE REF TO %s.
CREATE OBJECT shape.
CREATE OBJECT input.
input->tag = %s=>tag_object.
input->put( p0 = 'items' p1 = VALUE #( ) ).
shape->%s = input.
CREATE OBJECT input.
input->tag = %s=>tag_ref.
input->oval = shape.
CREATE OBJECT graph.
graph->tag = %s=>tag_array.
input->put( p0 = 'items' p1 = graph ).
CREATE OBJECT value.
value->tag = %s=>tag_string.
value->sval = 'one'.
shape->%s->push( value ).
cl_abap_unit_assert=>assert_equals( act = lines( graph->items ) exp = 1 ).
graph->append( value ).
cl_abap_unit_assert=>assert_equals( act = shape->%s->length( ) exp = 2 ).
ENDMETHOD.
ENDCLASS.
`, owner, d, d, d, d, names.Get("view.ownKeys"), d, d, d, names.Get("member.items"), names.Get("member.items"))
	storageHostFixture(t, files, owner)
}
