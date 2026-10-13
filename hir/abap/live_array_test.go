package abap

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLiveArrayStorageFixtures(t *testing.T) {
	const oracle = `const graph={a:[{flag:true,extra:'kept'}]};const a=graph.a,b=graph.a;let ok=a===b&&a[0]===b[0];a[0].flag=false;ok=ok&&!b[0].flag;const old=a[0];a[0]={flag:true};ok=ok&&b[0]!==old&&!old.flag;a.push(old);ok=ok&&b.length===2&&b[1]===old;delete a[0];ok=ok&&!(0 in b)&&b.length===2;a[0]=undefined;ok=ok&&(0 in b)&&b[0]===undefined;graph.a=[];ok=ok&&a.length===2&&graph.a!==a;const d=[null,undefined];delete d[1];d[3]=undefined;ok=ok&&d[0]===null&&!(1 in d)&&!(2 in d)&&(3 in d);console.log(ok);`
	out, err := exec.Command("node", "-e", oracle).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	dyn, child, parent := hir.T(hir.Dynamic), hir.Ref("ArrayChild"), hir.Ref("ArrayParent")
	arr := hir.T(hir.Array, child)
	field := func(x *hir.Expr, name string, ty hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, Type: ty, X: x, Name: name}
	}
	index := func(a *hir.Expr, i int) *hir.Expr { return &hir.Expr{Kind: hir.IndexGet, Type: child, X: a, Y: lit(i)} }
	graph := local("graph", dyn)
	a, b := local("a", arr), local("b", arr)
	body := hir.B(decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{"a":[{"flag":true,"extra":"kept"}]}`), dyn)), decl("parent", parent, rt("dynamic.classView", graph, parent)), decl("a", arr, field(local("parent", parent), "a", arr)), decl("b", arr, field(local("parent", parent), "a", arr)), decl("old", child, index(a, 0)), decl("ok", boolean, binary("==", a, b, boolean)), &hir.Stmt{Kind: hir.Assign, X: field(local("old", child), "flag", boolean), Y: hir.L(boolean, false)})
	check := func(x *hir.Expr) {
		body.List = append(body.List, assign("ok", boolean, binary("&&", local("ok", boolean), x, boolean)))
	}
	no := func(x *hir.Expr) *hir.Expr { return &hir.Expr{Kind: hir.Unary, Type: boolean, Op: "!", X: x} }
	check(no(field(index(b, 0), "flag", boolean)))
	check(binary("==", rt("array.indexOf", a, i32, local("old", child)), lit(0), boolean))
	check(rt("array.includes", b, boolean, local("old", child)))
	check(binary("==", rt("dynamic.asString", rt("dynamic.get", rt("dynamic.of", local("old", child), dyn), dyn, hir.L(str, "extra")), str), hir.L(str, "kept"), boolean))
	body.List = append(body.List, decl("fresh", child, rt("dynamic.classView", rt("json.parseSubset", hir.L(str, `{"flag":true}`), dyn), child)), &hir.Stmt{Kind: hir.Assign, X: index(a, 0), Y: local("fresh", child)})
	check(binary("==", index(b, 0), local("fresh", child), boolean))
	check(no(field(local("old", child), "flag", boolean)))
	body.List = append(body.List, run(rt("array.push", a, i32, local("old", child))))
	check(binary("==", rt("array.length", b, i32), lit(2), boolean))
	check(binary("==", index(b, 1), local("old", child), boolean))
	check(rt("array.delete", a, boolean, lit(0)))
	check(no(rt("array.has", b, boolean, lit(0))))
	body.List = append(body.List, run(rt("dynamic.put", rt("dynamic.of", a, dyn), hir.T(hir.Void), hir.L(str, "0"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn))))
	check(rt("array.has", b, boolean, lit(0)))
	check(binary("==", rt("dynamic.typeofValue", rt("dynamic.of", index(b, 0), dyn), str), hir.L(str, "undefined"), boolean))
	body.List = append(body.List, run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "a"), rt("json.parseSubset", hir.L(str, `[]`), dyn))), decl("replacement", arr, field(local("parent", parent), "a", arr)))
	check(binary("!=", local("replacement", arr), a, boolean))
	check(binary("==", rt("array.length", a, i32), lit(2), boolean))
	// A dynamic view preserves null independently of undefined and holes.
	darr := hir.T(hir.Array, hir.Ref(hir.RootObject))
	da := local("da", darr)
	body.List = append(body.List, decl("dg", dyn, rt("json.parseSubset", hir.L(str, `[null,null]`), dyn)), decl("da", darr, rt("dynamic.arrayView", local("dg", dyn), darr)))
	didx := func(i int) *hir.Expr {
		return rt("dynamic.of", &hir.Expr{Kind: hir.IndexGet, Type: hir.Ref(hir.RootObject), X: da, Y: lit(i)}, dyn)
	}
	check(rt("dynamic.strictEquals", didx(0), boolean, rt("dynamic.null", lit(0), dyn)))
	check(rt("array.delete", da, boolean, lit(1)))
	body.List = append(body.List, run(rt("dynamic.put", local("dg", dyn), hir.T(hir.Void), hir.L(str, "3"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn))))
	check(no(rt("array.has", da, boolean, lit(1))))
	check(no(rt("array.has", da, boolean, lit(2))))
	check(rt("array.has", da, boolean, lit(3)))
	check(binary("==", rt("dynamic.typeofValue", didx(3), str), hir.L(str, "undefined"), boolean))
	// Scalar aliases observe graph writes and writes through either view.
	sarr := hir.T(hir.Array, str)
	sa := local("sa", sarr)
	body.List = append(body.List, decl("sg", dyn, rt("json.parseSubset", hir.L(str, `["one","two"]`), dyn)), decl("sa", sarr, rt("dynamic.arrayView", local("sg", dyn), sarr)), decl("sb", sarr, rt("dynamic.arrayView", local("sg", dyn), sarr)), run(rt("dynamic.put", local("sg", dyn), hir.T(hir.Void), hir.L(str, "0"), rt("dynamic.of", hir.L(str, "changed"), dyn))))
	check(binary("==", &hir.Expr{Kind: hir.IndexGet, Type: str, X: sa, Y: lit(0)}, hir.L(str, "changed"), boolean))
	body.List = append(body.List, run(rt("array.unshift", sa, i32, hir.L(str, "head"))))
	check(binary("==", rt("array.length", local("sb", sarr), i32), lit(3), boolean))
	body.List = append(body.List, decl("shifted", hir.T(hir.Optional, str), rt("array.shift", local("sb", sarr), hir.T(hir.Optional, str))))
	check(binary("==", &hir.Expr{Kind: hir.Narrow, Type: str, X: local("shifted", hir.T(hir.Optional, str))}, hir.L(str, "head"), boolean))
	check(rt("object.has", local("parent", parent), boolean, hir.L(str, "a")))
	check(rt("dynamic.delete", graph, boolean, hir.L(str, "a")))
	check(no(rt("object.has", local("parent", parent), boolean, hir.L(str, "a"))))
	body.List = append(body.List, ret(local("ok", boolean)))
	m := method("check", boolean, body)
	m.Static = true
	files, names, err := EmitNamed(&hir.Program{Classes: []*hir.Class{{Name: "ArrayChild", Structural: true, Fields: []hir.Field{{Name: "flag", Type: boolean}}}, {Name: "ArrayParent", Structural: true, Fields: []hir.Field{{Name: "a", Type: arr}}}, {Name: "ArrayViews", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("ArrayViews")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_array DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_array IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
