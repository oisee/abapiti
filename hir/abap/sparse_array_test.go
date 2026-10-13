package abap

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"os/exec"
	"testing"
)

func TestSparseStorageFixtures(t *testing.T) {
	oracle := `const a=new Array(3);let ok=a.length===3&&!(0 in a);a[2]=undefined;ok=ok&&(2 in a)&&a[2]===undefined;delete a[2];ok=ok&&!(2 in a)&&a.length===3;a[5]=42;ok=ok&&!(3 in a)&&!(4 in a)&&(5 in a);a[1]=0;a.reverse();ok=ok&&(0 in a)&&!(1 in a)&&(4 in a);const b=a.slice();delete b[0];ok=ok&&(0 in a)&&!(0 in b);console.log(ok);`
	out, err := exec.Command("node", "-e", oracle).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	opt := hir.T(hir.Optional, i32)
	arr := hir.T(hir.Array, opt)
	a := local("a", arr)
	body := hir.B(decl("a", arr, &hir.Expr{Kind: hir.New, Type: arr, Args: []*hir.Expr{lit(3)}}), decl("ok", boolean, hir.L(boolean, true)))
	check := func(x *hir.Expr) {
		body.List = append(body.List, assign("ok", boolean, binary("&&", local("ok", boolean), x, boolean)))
	}
	has := func(x *hir.Expr, i int) *hir.Expr { return rt("array.has", x, boolean, lit(i)) }
	no := func(x *hir.Expr) *hir.Expr { return &hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: x} }
	set := func(i int, v *hir.Expr) {
		body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.IndexGet, Type: opt, X: a, Y: lit(i)}, Y: v})
	}
	check(binary("==", rt("array.length", a, i32), lit(3), boolean))
	check(no(has(a, 0)))
	set(2, hir.L(opt, nil))
	check(has(a, 2))
	check(binary("==", rt("array.get", a, opt, lit(2)), hir.L(opt, nil), boolean))
	check(rt("array.delete", a, boolean, lit(2)))
	check(no(has(a, 2)))
	check(binary("==", rt("array.length", a, i32), lit(3), boolean))
	check(rt("array.delete", a, boolean, lit(-1)))
	check(rt("array.delete", a, boolean, lit(100)))
	check(no(has(a, -1)))
	check(no(has(a, 100)))
	set(5, lit(42))
	check(no(has(a, 3)))
	check(no(has(a, 4)))
	check(has(a, 5))
	set(1, lit(0))
	body.List = append(body.List, run(rt("array.reverse", a, arr)))
	check(has(a, 0))
	check(no(has(a, 1)))
	check(has(a, 4))
	body.List = append(body.List, decl("b", arr, rt("array.slice0", a, arr)))
	check(rt("array.delete", local("b", arr), boolean, lit(0)))
	check(has(a, 0))
	check(no(has(local("b", arr), 0)))
	check(binary("==", rt("array.pop", a, opt), hir.L(opt, nil), boolean))
	check(binary("==", rt("array.length", a, i32), lit(5), boolean))
	body.List = append(body.List, ret(local("ok", boolean)))
	m := method("check", boolean, body)
	m.Static = true
	ref := hir.Ref("SparseNode")
	refs := hir.T(hir.Array, ref)
	r := local("nodes", refs)
	fill := hir.B(decl("nodes", refs, &hir.Expr{Kind: hir.New, Type: refs, Args: []*hir.Expr{lit(3)}}))
	for i := 2; i >= 0; i-- {
		fill.List = append(fill.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.IndexGet, Type: ref, X: r, Y: lit(i)}, Y: newObj(ref)})
	}
	fill.List = append(fill.List, ret(binary("&&", has(r, 0), binary("&&", has(r, 1), has(r, 2), boolean), boolean)))
	back := method("backwards", boolean, fill)
	back.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "SparseNode"}, {Name: "SparseStorage", Methods: []*hir.Method{m, back}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("SparseStorage")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_sparse DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_sparse IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"), owner, names.Get("member.backwards"))
	storageHostFixture(t, files, owner)
}
