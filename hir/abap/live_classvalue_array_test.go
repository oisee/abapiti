package abap

import (
	"fmt"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLiveClassValueArrayDescriptors(t *testing.T) {
	dyn, cv := hir.T(hir.Dynamic), hir.T(hir.ClassValue)
	array := hir.T(hir.Array, cv)
	descriptor := method("descriptor", cv, ret(&hir.Expr{Kind: hir.ClassOf, Owner: "DescriptorClass", Type: cv}))
	descriptor.Static = true
	activate := method("activate", array, ret(rt("dynamic.asRef", local("input", dyn), array)))
	activate.Static = true
	activate.Params = []hir.Param{{Name: "input", Type: dyn}}
	input := rt("dynamic.asRef", local("input", dyn), array)
	read := rt("array.get", input, hir.T(hir.Optional, cv), lit(0))
	// Foreach also reads boxed descriptors from the shared erased reference table.
	iterate := method("iterate", str, hir.B(&hir.Stmt{Kind: hir.ForEach, Name: "item", Type: cv, X: input, Body: ret(rt("classvalue.name", local("item", cv), str))}, ret(hir.L(str, "empty"))))
	iterate.Static = true
	iterate.Params = activate.Params
	same := method("same", boolean, ret(binary("==", &hir.Expr{Kind: hir.Narrow, Type: cv, X: read}, &hir.Expr{Kind: hir.ClassOf, Owner: "DescriptorClass", Type: cv}, boolean)))
	same.Static = true
	same.Params = activate.Params
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "DescriptorClass"}, {Name: "DescriptorProbe", Methods: []*hir.Method{descriptor, activate, iterate, same}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner, d, arr, desc := names.Get("DescriptorProbe"), names.Get("runtime.dynamic"), names.Get("runtime."+arrayStorage(array).String()), names.Get("runtime.classvalue")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_descriptors DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_descriptors IMPLEMENTATION.
METHOD check.
DATA graph TYPE REF TO %s.
DATA value TYPE REF TO %s.
DATA a TYPE REF TO %s.
DATA raw TYPE REF TO object.
DATA descriptor TYPE REF TO %s.
DATA actual TYPE string.
DATA same TYPE abap_bool.
DATA refused TYPE abap_bool.
CREATE OBJECT graph.
graph->tag = %s=>tag_array.
CREATE OBJECT value.
value->tag = %s=>tag_class.
value->cval = %s=>%s( ).
graph->append( value ).
a = %s=>%s( graph ).
descriptor = value->cval.
READ TABLE a->items INDEX 1 INTO raw.
value ?= raw.
cl_abap_unit_assert=>assert_true( xsdbool( descriptor = value->cval ) ).
same = %s=>%s( graph ).
cl_abap_unit_assert=>assert_true( same ).
actual = %s=>%s( graph ).
cl_abap_unit_assert=>assert_equals( act = actual exp = 'DescriptorClass' ).
a->save( ).
READ TABLE graph->items INDEX 1 INTO raw.
value ?= raw.
cl_abap_unit_assert=>assert_true( xsdbool( descriptor = value->cval ) ).
value->tag = %s=>tag_number.
TRY.
a = %s=>%s( graph ).
CATCH cx_sy_move_cast_error.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
CLEAR graph->items.
graph->append( VALUE #( ) ).
a = %s=>%s( graph ).
READ TABLE a->items INDEX 1 INTO raw.
cl_abap_unit_assert=>assert_true( xsdbool( raw IS NOT BOUND ) ).
ENDMETHOD.
ENDCLASS.
`, d, d, arr, desc, d, d, owner, names.Get("member.descriptor"), owner, names.Get("member.activate"), owner, names.Get("member.same"), owner, names.Get("member.iterate"), d, owner, names.Get("member.activate"), owner, names.Get("member.activate"))
	storageHostFixture(t, files, owner)
}
