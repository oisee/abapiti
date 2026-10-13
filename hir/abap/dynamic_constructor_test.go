package abap

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestDynamicNominalConstructorAndName(t *testing.T) {
	oracle, err := exec.Command("node", "-e", `class Base{}class Child extends Base{}let o=new Child;let record={constructor:"own"};if(o.constructor!==Child||o.constructor.name!==Child.name||record.constructor!=="own")process.exit(1);process.stdout.write(o.constructor.name)`).Output()
	if err != nil {
		t.Fatal(err)
	}
	dyn, cv := hir.T(hir.Dynamic), hir.T(hir.ClassValue)
	ctor := func(input *hir.Expr) *hir.Expr { return rt("dynamic.get", input, dyn, hir.L(str, "constructor")) }
	name := func(input *hir.Expr) *hir.Expr {
		return rt("dynamic.recordString", rt("dynamic.get", input, dyn, hir.L(str, "name")), str)
	}
	obj := rt("dynamic.of", newObj(hir.Ref("Child")), dyn)
	makeMethod := func(n string, result hir.Type, expr *hir.Expr) *hir.Method {
		m := method(n, result, ret(expr))
		m.Static = true
		return m
	}
	staticValue := func(n string) *hir.Expr {
		return rt("dynamic.of", &hir.Expr{Kind: hir.ClassOf, Owner: n, Type: cv}, dyn)
	}
	fields := []hir.Field{{Name: "constructor", Type: str}}
	ownCtor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("OwnRecord")}, Name: "constructor", Type: str}, Y: hir.L(str, "own")}))
	methods := []*hir.Method{
		makeMethod("run", str, name(ctor(obj))),
		makeMethod("descriptor", str, name(staticValue("Child"))),
		makeMethod("identity", boolean, binary("==", rt("dynamic.asClassValue", ctor(obj), cv), &hir.Expr{Kind: hir.ClassOf, Owner: "Child", Type: cv}, boolean)),
		makeMethod("record", str, rt("dynamic.recordString", ctor(rt("dynamic.of", newObj(hir.Ref("OwnRecord")), dyn)), str)),
		makeMethod("shadowName", str, name(staticValue("ShadowName"))),
		makeMethod("unsafe", str, name(ctor(rt("dynamic.of", newObj(hir.Ref("UnsafeCtor")), dyn)))),
	}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Base"}, {Name: "Child", Super: "Base"}, {Name: "OwnRecord", Structural: true, Fields: fields, Ctor: ownCtor}, {Name: "ShadowName", Fields: []hir.Field{{Name: "name", Type: str, Static: true}}}, {Name: "UnsafeCtor", CoercionUnsafe: true}, {Name: "ConstructorProbe", Methods: methods}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("ConstructorProbe")
	unit := ("CLASS ltcl_ctor DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_ctor IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA identical TYPE abap_bool.\nDATA refused TYPE abap_bool.\n")
	for _, entry := range []struct{ name, want string }{{"run", string(oracle)}, {"descriptor", string(oracle)}, {"record", "own"}} {
		unit += fmt.Sprintf("actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\n", owner, names.Get("member."+entry.name), entry.want)
	}
	unit += fmt.Sprintf("identical = %s=>%s( ).\ncl_abap_unit_assert=>assert_true( identical ).\n", owner, names.Get("member.identity"))
	for _, m := range []string{"shadowName", "unsafe"} {
		unit += fmt.Sprintf("CLEAR refused.\nTRY.\nactual = %s=>%s( ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member."+m), names.Get("class.refusal"))
	}
	unit += "ENDMETHOD.\nENDCLASS.\n"
	files[owner+".clas.testclasses.abap"] = unit
	storageHostFixture(t, files, owner)
}
