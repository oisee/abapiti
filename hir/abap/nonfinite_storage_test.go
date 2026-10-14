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
	p := &hir.Program{Classes: []*hir.Class{{Name: "Box", Fields: []hir.Field{{Name: "value", Type: num}}, Ctor: ctor, Methods: []*hir.Method{getter}}, {Name: "Probe", Methods: []*hir.Method{check}}}}
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
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
