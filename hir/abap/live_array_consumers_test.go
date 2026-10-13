package abap

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestLiveArrayConsumerFixtures(t *testing.T) {
	dyn, root := hir.T(hir.Dynamic), hir.Ref(hir.RootObject)
	holder := hir.Ref("ArgumentHolder")
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: str, X: &hir.Expr{Kind: hir.This, Type: holder}, Name: "value"}, Y: local("value", str)}))
	ctor.Params = []hir.Param{{Name: "value", Type: str}}
	raw := hir.T(hir.Array, root)
	body := hir.B(decl("graph", dyn, rt("json.parseSubset", hir.L(str, `["old"]`), dyn)), decl("args", raw, rt("dynamic.arrayView", local("graph", dyn), raw)), run(rt("dynamic.put", local("graph", dyn), hir.T(hir.Void), hir.L(str, "0"), rt("dynamic.of", hir.L(str, "fresh"), dyn))), decl("made", holder, rt("classvalue.newArgs", &hir.Expr{Kind: hir.ClassOf, Type: hir.T(hir.ClassValue), Owner: "ArgumentHolder"}, holder, &hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.Array, dyn), X: local("args", raw)})), ret(binary("==", &hir.Expr{Kind: hir.FieldGet, Type: str, X: local("made", holder), Name: "value"}, hir.L(str, "fresh"), boolean)))
	m := method("check", boolean, body)
	m.Static = true
	snapshot := hir.Ref("ArraySnapshot")
	items := hir.T(hir.Array, holder)
	snapshotCtor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: items, X: &hir.Expr{Kind: hir.This, Type: snapshot}, Name: "items"}, Y: local("items", items)}))
	snapshotCtor.Params = []hir.Param{{Name: "items", Type: items}}
	project := method("project", snapshot, ret(rt("dynamic.materialize", local("graph", dyn), snapshot)))
	project.Static = true
	project.Params = []hir.Param{{Name: "graph", Type: dyn}}
	files, names, err := EmitNamed(&hir.Program{Classes: []*hir.Class{{Name: "ArgumentHolder", Structural: true, Ctor: ctor, Fields: []hir.Field{{Name: "value", Type: str}}}, {Name: "ArraySnapshot", Structural: true, Ctor: snapshotCtor, Fields: []hir.Field{{Name: "items", Type: items}}}, {Name: "ArrayConsumers", Methods: []*hir.Method{m, project}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner, d := names.Get("ArrayConsumers"), names.Get("runtime.dynamic")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_consumers DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_consumers IMPLEMENTATION.
METHOD check.
cl_abap_unit_assert=>assert_true( %s=>%s( ) ).
DATA graph TYPE REF TO %s.
DATA value TYPE REF TO %s.
DATA array TYPE REF TO %s.
DATA projected TYPE %s.
DATA refused TYPE abap_bool.
CREATE OBJECT graph.
graph->tag = %s=>tag_object.
CREATE OBJECT array.
array->tag = %s=>tag_array.
CREATE OBJECT value.
value->tag = %s=>tag_object.
array->put( p0 = '2' p1 = value ).
graph->put( p0 = 'items' p1 = array ).
TRY.
projected = %s=>%s( graph ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
cl_abap_unit_assert=>assert_equals( act = lines( array->items ) exp = 3 ).
ENDMETHOD.
ENDCLASS.
`, owner, names.Get("member.check"), d, d, d, "REF TO "+names.Get("ArraySnapshot"), d, d, d, owner, names.Get("member.project"), names.Get("exception.RegistryJSONSubsetError"))
	storageHostFixture(t, files, owner)
}
