package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Virtual/super HIR calls need not carry Owner. Their declared Number
// results must follow the callee's tagged storage, including inherited and
// interface dispatch; otherwise ABAP emits a reference MOVE into TYPE f.
func TestNonfiniteReceiverCallStorage(t *testing.T) {
	for _, dispatch := range []string{"direct", "virtual", "inherited", "interface", "inherited-interface", "super"} {
		t.Run(dispatch, func(t *testing.T) {
			num := hir.T(hir.Number)
			read := method("read", num, ret(rt("string.parseFloatNumber", hir.L(str, "NaN"), num)))
			read.Virtual = true
			base := &hir.Class{Name: "Base", Methods: []*hir.Method{read}}
			child := &hir.Class{Name: "Child", Super: "Base"}
			p := &hir.Program{Classes: []*hir.Class{base, child}}
			call := &hir.Expr{Kind: hir.VirtualCall, Name: "read", Type: num, X: &hir.Expr{Kind: hir.New, Type: hir.Ref("Base")}}
			switch dispatch {
			case "direct":
				read.Virtual = false
				call.Kind, call.Owner = hir.DirectCall, "Base"
			case "inherited":
				call.X.Type = hir.Ref("Child")
			case "interface", "inherited-interface":
				view := hir.Type{Kind: hir.InterfaceRef, Name: "Source"}
				if dispatch == "inherited-interface" {
					child.Implements = []string{"Source"}
					call.X.Type = hir.Ref("Child")
				} else {
					base.Implements = []string{"Source"}
				}
				p.Interfaces = []*hir.Interface{{Name: "Source", Methods: []*hir.Method{method("read", num, nil)}}}
				call.X = &hir.Expr{Kind: hir.Cast, Type: view, X: call.X}
			case "super":
				forward := method("forward", num, ret(&hir.Expr{Kind: hir.SuperCall, Name: "read", Type: num}))
				forward.Virtual = true
				child.Methods = []*hir.Method{forward}
				call.Name, call.X.Type = "forward", hir.Ref("Child")
			}
			probe := method("value", num, ret(call))
			probe.Static = true
			check := method("check", boolean, ret(rt("number.isNaN", &hir.Expr{Kind: hir.DirectCall, Owner: "Probe", Name: "value", Type: num}, boolean)))
			check.Static = true
			p.Classes = append(p.Classes, &hir.Class{Name: "Probe", Methods: []*hir.Method{probe, check}})
			files, names, err := emitABAPNamed(t, p)
			if err != nil {
				t.Fatal(err)
			}
			if !call.Type.Equal(num) || !probe.Result.Equal(num) {
				t.Fatal("call storage lowering mutated source HIR")
			}
			owner := names.Get("Probe")
			boxed := "TYPE REF TO " + names.Get("runtime."+hir.T(hir.Optional, num).String())
			if !strings.Contains(files[owner+".clas.abap"], "RETURNING VALUE(result) "+boxed) {
				t.Fatalf("receiver call result remained a native float:\n%s", files[owner+".clas.abap"])
			}
			files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
			storageHostFixture(t, files, owner)
		})
	}
}

func TestNonfiniteScalarStorage(t *testing.T) {
	out, err := exec.Command("node", "-e", `class Box { constructor(value) { this.value=value; } get() { return this.value; } } const x=new Box(undefined+1+0); console.log(String(x.get()),Number.isNaN(x.get()),x.get()===x.get(),x.get()<0,Math.trunc(x.get())||0);`).CombinedOutput()
	if err != nil || string(out) != "NaN true false false 0\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	num, ref := hir.T(hir.Number), hir.Ref("Box")
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: num, X: &hir.Expr{Kind: hir.This, Type: ref}, Name: "value"}, Y: local("value", num)}))
	ctor.Params = []hir.Param{{Name: "value", Type: num}}
	getter := method("get", num, ret(&hir.Expr{Kind: hir.FieldGet, Type: num, X: &hir.Expr{Kind: hir.This, Type: ref}, Name: "value"}))
	optional := hir.T(hir.Optional, num)
	operand := &hir.Expr{Kind: hir.Conditional, Type: num,
		X: &hir.Expr{Kind: hir.IsUndefined, Type: boolean, X: local("absent", optional)},
		Y: rt("string.parseFloatNumber", hir.L(str, "NaN"), num),
		Z: &hir.Expr{Kind: hir.Narrow, Type: num, X: local("absent", optional)},
	}
	value := binary("+", binary("+", operand, hir.L(num, 1), num), hir.L(num, 0), num)
	call := func() *hir.Expr {
		return &hir.Expr{Kind: hir.DirectCall, Owner: "Box", Name: "get", Type: num, X: local("box", ref)}
	}
	checks := []*hir.Expr{
		rt("number.isNaN", call(), boolean),
		binary("!=", call(), call(), boolean),
		binary("==", rt("number.index", call(), i32), lit(0), boolean),
		binary("==", rt("number.toString", call(), str), hir.L(str, "NaN"), boolean),
		&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: binary("<", call(), hir.L(num, 0), boolean)},
		binary("==", &hir.Expr{Kind: hir.DirectCall, Owner: "Box", Name: "get", Type: num, X: local("finite", ref)}, hir.L(num, 3), boolean),
	}
	ok := checks[0]
	for _, next := range checks[1:] {
		ok = binary("&&", ok, next, boolean)
	}
	check := method("check", boolean, hir.B(
		decl("absent", optional, hir.L(optional, nil)),
		decl("box", ref, &hir.Expr{Kind: hir.New, Type: ref, Args: []*hir.Expr{value}}),
		decl("finite", ref, &hir.Expr{Kind: hir.New, Type: ref, Args: []*hir.Expr{hir.L(num, 3)}}),
		ret(ok),
	))
	check.Static = true
	dyn := hir.T(hir.Dynamic)
	erase := method("erase", boolean, hir.B(
		decl("box", ref, &hir.Expr{Kind: hir.New, Type: ref, Args: []*hir.Expr{hir.L(num, 3)}}),
		ret(rt("dynamic.delete", rt("dynamic.of", local("box", ref), dyn), boolean, hir.L(str, "value"))),
	))
	erase.Static = true
	p := &hir.Program{Classes: []*hir.Class{{Name: "Box", Fields: []hir.Field{{Name: "value", Type: num}}, Ctor: ctor, Methods: []*hir.Method{getter}}, {Name: "Probe", Methods: []*hir.Method{check, erase}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Classes[0].Fields[0].Type.Equal(num) {
		t.Fatal("storage lowering mutated source HIR")
	}
	if !strings.Contains(files[names.Get("Box")+".clas.abap"], "TYPE "+"REF TO "+names.Get("runtime."+hir.T(hir.Optional, num).String())) {
		t.Fatal("nonfinite field was not boxed")
	}
	owner := names.Get("Probe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\nDATA failure TYPE REF TO %s.\nDATA ignored TYPE abap_bool.\nDATA refused TYPE abap_bool.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nTRY.\nignored = %s=>%s( ).\nCATCH %s INTO failure.\ncl_abap_unit_assert=>assert_equals( act = failure->get_text( ) exp = `ABAP view refused: record_storage.go: nativeDelete: required or prototype slot cannot retain absence` ).\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.\nENDCLASS.\n", names.Get("view.refusal"), owner, names.Get("member.check"), owner, names.Get("member.erase"), names.Get("view.refusal"))
	storageHostFixture(t, files, owner)
}

// With two arms per part, B's arm (most-derived first) and A's land in
// different parts: deleting B's own key must still reach B's arm.
func TestNativeDeleteParts(t *testing.T) {
	out, err := exec.Command("node", "-e", `class A { opt; } class B extends A { bopt; } class C { x; } const b=new B(); b.opt="1"; b.bopt="2"; const c=new C(); c.x="3"; const r=[delete b.bopt, delete b.opt, delete b.zzz, delete c.x]; console.log(r.every(Boolean) && b.bopt===undefined && b.opt===undefined && c.x===undefined);`).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	arms := nativeDeleteArms
	nativeDeleteArms = 2
	t.Cleanup(func() { nativeDeleteArms = arms })
	opt, dyn := hir.T(hir.Optional, str), hir.T(hir.Dynamic)
	field := func(owner, name, of string) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, Type: opt, X: local(of, hir.Ref(owner)), Name: name}
	}
	set := func(owner, name, of, value string) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, X: field(owner, name, of), Y: hir.L(opt, value)}
	}
	remove := func(of, owner, key string) *hir.Expr {
		return rt("dynamic.delete", rt("dynamic.of", local(of, hir.Ref(owner)), dyn), boolean, hir.L(str, key))
	}
	ok := remove("b", "B", "bopt")
	for _, next := range []*hir.Expr{remove("b", "B", "opt"), remove("b", "B", "zzz"), remove("c", "C", "x"),
		{Kind: hir.IsUndefined, Type: boolean, X: field("B", "bopt", "b")}, {Kind: hir.IsUndefined, Type: boolean, X: field("B", "opt", "b")}, {Kind: hir.IsUndefined, Type: boolean, X: field("C", "x", "c")}} {
		ok = binary("&&", ok, next, boolean)
	}
	check := method("check", boolean, hir.B(
		decl("b", hir.Ref("B"), newObj(hir.Ref("B"))), set("B", "opt", "b", "1"), set("B", "bopt", "b", "2"),
		decl("c", hir.Ref("C"), newObj(hir.Ref("C"))), set("C", "x", "c", "3"),
		ret(ok),
	))
	check.Static = true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "A", Fields: []hir.Field{{Name: "opt", Type: opt}}},
		{Name: "B", Super: "A", Fields: []hir.Field{{Name: "bopt", Type: opt}}},
		{Name: "C", Fields: []hir.Field{{Name: "x", Type: opt}}},
		{Name: "Probe", Methods: []*hir.Method{check}},
	}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	deletion := files[names.Get("runtime.nativeDelete")+".clas.abap"]
	if !strings.Contains(deletion, "METHOD run_1.") || !strings.Contains(deletion, "run_1( EXPORTING p0 = p0 key = key IMPORTING handled = found result = result ).") {
		t.Fatalf("native delete was not split into parts:\n%s", deletion)
	}
	if strings.Index(deletion, "IS INSTANCE OF "+names.Get("B")+".") > strings.Index(deletion, "IS INSTANCE OF "+names.Get("A")+".") {
		t.Fatal("subclass arm must precede its base")
	}
	owner := names.Get("Probe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
