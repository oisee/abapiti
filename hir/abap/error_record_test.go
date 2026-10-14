package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// builtin.Error is rewritten by support(), after ordinary classes are emitted.
// Its subclasses must retain the same deletion/presence ABI as other roots.
func TestErrorSubclassRecordMask(t *testing.T) {
	out, err := exec.Command("node", "-e", `class Skip extends Error { constructor(issue) { super(); this.issue=issue; } }; const x=new Skip("kept"); delete x.issue; x.issue="restored"; console.log(Object.hasOwn(x,"issue") && x.issue==="restored");`).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	child, dyn := hir.Ref("Skip"), hir.T(hir.Dynamic)
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: hir.T(hir.Optional, str), X: &hir.Expr{Kind: hir.This, Type: child}, Name: "issue"}, Y: hir.L(str, "kept")}))
	m := method("check", boolean, hir.B(
		decl("x", child, newObj(child)),
		run(rt("dynamic.delete", rt("dynamic.of", local("x", child), dyn), boolean, hir.L(str, "issue"))),
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: hir.T(hir.Optional, str), X: local("x", child), Name: "issue"}, Y: hir.L(str, "restored")},
		ret(rt("object.hasOwn", local("x", child), boolean, hir.L(str, "issue"))),
	))
	m.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{
		{Name: "builtin.Error", Fields: []hir.Field{{Name: "message", Type: str}}},
		{Name: "Skip", Super: "builtin.Error", Fields: []hir.Field{{Name: "issue", Type: hir.T(hir.Optional, str)}}, Ctor: ctor},
		{Name: "Probe", Methods: []*hir.Method{m}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	mask := names.Get("record.deleted")
	if !strings.Contains(files[names.Get("builtin.Error")+".clas.abap"], "DATA "+mask+" TYPE HASHED TABLE") {
		t.Fatal("runtime Error lost inherited record mask")
	}
	if !strings.Contains(files[names.Get("Skip")+".clas.abap"], "DELETE me->"+mask) {
		t.Fatal("fixture did not exercise inherited mask write")
	}
	owner := names.Get("Probe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
