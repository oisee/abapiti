package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLiveOptionalReferenceIdentity(t *testing.T) {
	ref, dyn := hir.Ref("Shape"), hir.T(hir.Dynamic)
	opt := hir.T(hir.Optional, ref)
	view := func() *hir.Expr { return rt("dynamic.classView", local("graph", dyn), ref) }
	body := hir.B(decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{"value":"same"}`), dyn)), decl("left", opt, view()), decl("right", opt, view()), ret(binary("==", local("left", opt), local("right", opt), boolean)))
	m := method("check", boolean, body)
	m.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "Shape", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}, {Name: "Probe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("Probe")
	if !strings.Contains(files[owner+".clas.abap"], "->strict_equals(") {
		t.Fatal("optional references compared adapter identity instead of source identity")
	}
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
