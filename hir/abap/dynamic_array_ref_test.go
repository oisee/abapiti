package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Array.isArray narrows an any to any[] using dynamic.asRef (xmlToArray).
// Parsed arrays are graph nodes, not instances of the typed array wrapper.
func TestDynamicArrayRefUsesLiveStorage(t *testing.T) {
	const oracle = `const graph=JSON.parse('["one",null]');const a=graph,b=graph;let ok=a===b&&Array.isArray(a);a.push("tail");ok=ok&&b.length===3&&graph.length===3;graph[0]="changed";ok=ok&&b[0]==="changed";const native=["native"];const alias=native;alias.push("second");console.log(ok&&alias===native&&native.length===2);`
	out, err := exec.Command("node", "-e", oracle).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node array assertion oracle: %s %v", out, err)
	}
	dyn := hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, dyn)
	graph, a, b := local("graph", dyn), local("a", arr), local("b", arr)
	body := hir.B(
		decl("graph", dyn, rt("json.parseSubset", hir.L(str, `["one",null]`), dyn)),
		decl("a", arr, rt("dynamic.asRef", graph, arr)),
		decl("b", arr, rt("dynamic.asRef", graph, arr)),
		decl("ok", boolean, binary("==", a, b, boolean)),
		run(rt("array.push", a, i32, rt("dynamic.of", hir.L(str, "tail"), dyn))),
	)
	check := func(x *hir.Expr) {
		body.List = append(body.List, assign("ok", boolean, binary("&&", local("ok", boolean), x, boolean)))
	}
	check(rt("dynamic.isArray", rt("dynamic.of", a, dyn), boolean))
	check(binary("==", rt("array.length", b, i32), lit(3), boolean))
	check(binary("==", rt("dynamic.asNumber", rt("dynamic.get", graph, dyn, hir.L(str, "length")), hir.T(hir.Number)), hir.L(hir.T(hir.Number), float64(3)), boolean))
	body.List = append(body.List, run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "0"), rt("dynamic.of", hir.L(str, "changed"), dyn))))
	check(binary("==", rt("dynamic.asString", &hir.Expr{Kind: hir.IndexGet, Type: dyn, X: b, Y: lit(0)}, str), hir.L(str, "changed"), boolean))
	body.List = append(body.List,
		decl("native", arr, newObj(arr)),
		run(rt("array.push", local("native", arr), i32, rt("dynamic.of", hir.L(str, "native"), dyn))),
		decl("alias", arr, rt("dynamic.asRef", rt("dynamic.of", local("native", arr), dyn), arr)),
		run(rt("array.push", local("alias", arr), i32, rt("dynamic.of", hir.L(str, "second"), dyn))),
	)
	check(binary("==", local("alias", arr), local("native", arr), boolean))
	check(binary("==", rt("array.length", local("native", arr), i32), lit(2), boolean))
	body.List = append(body.List, ret(local("ok", boolean)))
	m := method("check", boolean, body)
	m.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "DynamicArrayRef", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("DynamicArrayRef")
	source := files[owner+".clas.abap"]
	if !strings.Contains(source, "->source = ") || strings.Contains(source, "->as_ref( )") {
		t.Fatalf("array assertion must adapt the graph, not cast as_ref: %s", source)
	}
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_array_ref DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_array_ref IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
