package abap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestChaiSizedArrayFixtures(t *testing.T) {
	root := os.Getenv("ABAPITI_TESTS_UPSTREAM")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		root = filepath.Join(home, "dev", "abaplint")
	}
	chai := filepath.Join(root, "node_modules", "chai")
	if _, err := os.Stat(chai); err != nil {
		chai = filepath.Join(root, "packages", "core", "node_modules", "chai")
	}
	if _, err := os.Stat(chai); err != nil {
		t.Skip("real Chai oracle not installed")
	}
	oracle, err := exec.Command("node", "-e", `const {expect}=require(process.argv[1]);expect(new Array(3)).length(3);expect(new Array(2).fill(0)).eql([0,0]);expect(new Array(1)).eql([undefined]);expect([undefined]).eql(new Array(1));expect(new Array(1)).not.include(undefined);expect(new Array(1)).not.deep.include(undefined);expect(new Array(1)).not.members([undefined]);console.log("ok")`, chai).CombinedOutput()
	if err != nil || string(oracle) != "ok\n" {
		t.Fatalf("Node/Chai: %s %v", oracle, err)
	}
	opt, dyn := hir.T(hir.Optional, i32), hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, opt)
	a, b := local("a", arr), local("b", arr)
	sized := func(n int) *hir.Expr { return &hir.Expr{Kind: hir.New, Type: arr, Args: []*hir.Expr{lit(n)}} }
	box := func(x *hir.Expr) *hir.Expr { return rt("chai.box", x, dyn) }
	length := method("length", i32, hir.B(decl("a", arr, sized(3)), ret(rt("array.length", a, i32))))
	set := func(x *hir.Expr, index, value *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.IndexGet, Type: opt, X: x, Y: index}, Y: value}
	}
	// The fill lowering uses indexed writes, not iteration (which skips holes).
	fill := method("fill", boolean, hir.B(decl("a", arr, sized(2)), decl("b", arr, newObj(arr)),
		set(a, lit(0), lit(0)), set(a, lit(1), lit(0)),
		run(rt("array.push", b, i32, lit(0))), run(rt("array.push", b, i32, lit(0))),
		ret(rt("chai.deepEqual", box(a), boolean, box(b)))))
	holes := method("holes", boolean, hir.B(decl("a", arr, sized(1)), decl("b", arr, newObj(arr)),
		run(rt("array.push", b, i32, hir.L(opt, nil))), ret(rt("chai.deepEqual", box(a), boolean, box(b)))))
	reverseHoles := method("reverseHoles", boolean, hir.B(decl("a", arr, sized(1)), decl("b", arr, newObj(arr)),
		run(rt("array.push", b, i32, hir.L(opt, nil))), ret(rt("chai.deepEqual", box(b), boolean, box(a)))))
	methods := []*hir.Method{length, fill, holes, reverseHoles}
	for _, op := range []string{"chai.arrayInclude", "chai.deepInclude", "chai.members"} {
		result := boolean
		arg := rt("chai.box", hir.L(opt, nil), dyn)
		var extra []*hir.Expr
		if op == "chai.members" {
			arg, extra = box(b), []*hir.Expr{hir.L(boolean, false)}
		} else if op == "chai.deepInclude" {
			result = i32
		}
		expression := rt(op, box(a), result, append([]*hir.Expr{arg}, extra...)...)
		if result.Kind == hir.I32 {
			expression = binary("==", expression, lit(1), boolean)
		}
		methods = append(methods, method(strings.TrimPrefix(op, "chai."), boolean, hir.B(
			decl("a", arr, sized(1)), decl("b", arr, newObj(arr)),
			run(rt("array.push", b, i32, hir.L(opt, nil))), ret(expression))))
	}
	// Callback loops must not confuse a reference hole with present undefined.
	iterate := method("iterate", boolean, hir.B(decl("a", arr, sized(1)),
		&hir.Stmt{Kind: hir.ForEach, Name: "v", Type: opt, X: a, Body: hir.B()}, ret(hir.L(boolean, true))))
	methods = append(methods, iterate)
	for _, m := range methods {
		m.Static = true
	}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "SizedChai", Methods: methods}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("SizedChai")
	src := files[owner+".clas.abap"]
	if strings.Contains(src, "hole-aware storage required") || !strings.Contains(src, "->allocate(") || !strings.Contains(src, "array.iteration: sparse callback ABI required") {
		t.Fatal("sized allocation must be allowed, sparse callbacks must refuse")
	}
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_sized DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_sized IMPLEMENTATION.\nMETHOD check.\nDATA answer TYPE abap_bool.\nDATA refused TYPE abap_bool.\n")
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = 3 ).\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\n", owner, names.Get("member.length"), owner, names.Get("member.fill"))
	// Chai 4 equates holes and undefined, but ABAP's dense adapter is not yet
	// certified for that observation: it must trap, never return a wrong bool.
	for _, name := range []string{"holes", "reverseHoles", "arrayInclude", "deepInclude", "members", "iterate"} {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nanswer = %s=>%s( ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member."+name), names.Get("exception.chaiRefusal"))
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
