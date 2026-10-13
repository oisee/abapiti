package abap

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestRecordStorageFixtures(t *testing.T) {
	const oracle = `const g={keep:true,u:undefined};let ok=('u' in g)&&g.u===undefined;ok=ok&&delete g.u&&!('u' in g)&&g.keep;g.u=undefined;ok=ok&&('u' in g)&&('toString' in g)&&!Object.hasOwn(g,'toString');console.log(ok);`
	out, err := exec.Command("node", "-e", oracle).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	dyn, root := hir.T(hir.Dynamic), hir.Ref(hir.RootObject)
	graph := local("graph", dyn)
	erased := func(d *hir.Expr) *hir.Expr { return rt("dynamic.asRef", d, root) }
	has := func(d *hir.Expr, k string) *hir.Expr { return rt("object.has", erased(d), boolean, hir.L(str, k)) }
	own := func(d *hir.Expr, k string) *hir.Expr { return rt("object.hasOwn", erased(d), boolean, hir.L(str, k)) }
	no := func(x *hir.Expr) *hir.Expr { return &hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: x} }
	put := func(d *hir.Expr, k string, v *hir.Expr) *hir.Stmt {
		return run(rt("dynamic.put", d, hir.T(hir.Void), hir.L(str, k), v))
	}
	del := func(d *hir.Expr, k string) *hir.Expr { return rt("dynamic.delete", d, boolean, hir.L(str, k)) }
	body := hir.B(decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{"keep":true}`), dyn)), put(graph, "u", rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)), decl("ok", boolean, own(graph, "u")))
	check := func(x *hir.Expr) {
		body.List = append(body.List, assign("ok", boolean, binary("&&", local("ok", boolean), x, boolean)))
	}
	check(binary("==", rt("dynamic.get", graph, dyn, hir.L(str, "u")), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn), boolean))
	check(del(graph, "u"))
	check(no(has(graph, "u")))
	check(own(graph, "keep"))
	check(del(graph, "missing"))
	body.List = append(body.List, put(graph, "u", rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)))
	check(own(graph, "u"))
	check(has(graph, "toString"))
	check(no(own(graph, "toString")))
	native := hir.Ref("RecordShape")
	keys := []string{"u"}
	body.List = append(body.List, decl("native", native, &hir.Expr{Kind: hir.New, Type: native, OwnKeys: &keys}), decl("boxed", dyn, rt("dynamic.of", local("native", native), dyn)))
	boxed := local("boxed", dyn)
	check(own(boxed, "u"))
	check(no(own(boxed, "flag")))
	check(del(boxed, "u"))
	check(no(own(boxed, "u")))
	body.List = append(body.List, put(boxed, "u", rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)), put(boxed, "extra", rt("dynamic.of", hir.L(str, "kept"), dyn)))
	check(own(boxed, "u"))
	check(own(boxed, "extra"))
	check(binary("==", rt("dynamic.asString", rt("dynamic.get", boxed, dyn, hir.L(str, "extra")), str), hir.L(str, "kept"), boolean))
	check(del(boxed, "extra"))
	check(no(own(boxed, "extra")))
	nominal := hir.Ref("NominalRecord")
	body.List = append(body.List, decl("nominal", nominal, newObj(hir.Ref("NominalRecordChild"))), decl("nd", dyn, rt("dynamic.of", local("nominal", nominal), dyn)))
	nomHas := func(k string) *hir.Expr {
		return rt("object.hasOwn", local("nominal", nominal), boolean, hir.L(str, k))
	}
	check(nomHas("u"))
	check(del(local("nd", dyn), "u"))
	check(no(nomHas("u")))
	check(del(local("nd", dyn), "absent"))
	body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: hir.T(hir.Optional, str), X: local("nominal", nominal), Name: "u"}, Y: hir.L(str, "restored")})
	check(nomHas("u"))
	roots := hir.T(hir.Array, root)
	body.List = append(body.List, decl("receivers", roots, newObj(roots)), run(rt("array.push", local("receivers", roots), i32, erased(graph))), run(rt("array.push", local("receivers", roots), i32, erased(boxed))), decl("found", i32, lit(0)), &hir.Stmt{Kind: hir.ForEach, Name: "receiver", Type: root, X: local("receivers", roots), Body: &hir.Stmt{Kind: hir.If, X: rt("object.has", local("receiver", root), boolean, hir.L(str, "keep")), Body: assign("found", i32, binary("+", local("found", i32), lit(1), i32))}})
	check(binary("==", local("found", i32), lit(1), boolean))
	body.List = append(body.List, ret(local("ok", boolean)))
	m := method("check", boolean, body)
	m.Static = true
	required := method("required", boolean, ret(del(boxed, "flag")))
	required.Static = true
	required.Params = []hir.Param{{Name: "boxed", Type: dyn}}
	activate := method("activate", native, ret(rt("dynamic.classView", local("input", dyn), native)))
	activate.Static = true
	activate.Params = []hir.Param{{Name: "input", Type: dyn}}
	files, names, err := EmitNamed(&hir.Program{Classes: []*hir.Class{{Name: "RecordShape", Structural: true, Fields: []hir.Field{{Name: "u", Type: hir.T(hir.Optional, str)}, {Name: "flag", Type: boolean}}}, {Name: "NominalRecordChild", Super: "NominalRecord"}, {Name: "NominalRecord", Fields: []hir.Field{{Name: "u", Type: hir.T(hir.Optional, str)}, {Name: "flag", Type: boolean}}}, {Name: "Records", Methods: []*hir.Method{m, required, activate}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner, d := names.Get("Records"), names.Get("runtime.dynamic")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_records DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_records IMPLEMENTATION.\nMETHOD check.\nDATA d TYPE REF TO %s.\nDATA refused TYPE abap_bool.\nDATA result TYPE abap_bool.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nCREATE OBJECT d.\nd->tag = %s=>tag_ref.\nd->oval = NEW %s( ).\nTRY.\nresult = %s=>%s( d ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.\nENDCLASS.\n", d, owner, names.Get("member.check"), d, names.Get("RecordShape"), owner, names.Get("member.required"), names.Get("view.refusal"))
	storageHostFixture(t, files, owner)
}
