package abap

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/abapfixture"
)

func TestNativeDeleteEmptyClassFixtures(t *testing.T) {
	out, err := exec.Command("node", "-e", "class Empty {} class Child extends Empty {} console.log(delete new Child().absent)").CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node delete oracle: %s %v", out, err)
	}
	dyn := hir.T(hir.Dynamic)
	check := method("check", boolean, ret(rt("dynamic.delete", rt("dynamic.of", newObj(hir.Ref("EmptyChild")), dyn), boolean, hir.L(str, "absent"))))
	check.Static = true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "EmptyNative", Fields: []hir.Field{{Name: "staticOnly", Type: i32, Static: true}}},
		{Name: "EmptyChild", Super: "EmptyNative"},
		{Name: "DeleteProbe", Methods: []*hir.Method{check}},
	}}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if issues := abapfixture.KernelViolations(files); len(issues) > 0 {
		t.Fatal(issues)
	}
	owner := names.Get("DeleteProbe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_delete DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_delete IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
