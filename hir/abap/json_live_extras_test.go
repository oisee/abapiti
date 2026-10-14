package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestJSONLiveNativeExtras(t *testing.T) {
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify([false,true].map(flag=>{const global={files:"/src/**"};global.applyUnspecifiedRules=flag;return JSON.stringify(global)})));`).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %s %v", out, err)
	}
	shape, dyn := hir.Ref("Global"), hir.T(hir.Dynamic)
	activate := method("activate", shape, ret(rt("dynamic.classView", local("graph", dyn), shape)))
	activate.Static = true
	activate.Params = []hir.Param{{Name: "graph", Type: dyn}}
	keys := []string{"files"}
	runMethod := method("run", str, hir.B(
		decl("global", shape, &hir.Expr{Kind: hir.New, Type: shape, Args: []*hir.Expr{hir.L(str, "/src/**")}, OwnKeys: &keys}),
		run(rt("dynamic.put", rt("dynamic.of", local("global", shape), dyn), hir.T(hir.Void), hir.L(str, "applyUnspecifiedRules"), rt("dynamic.of", local("flag", boolean), dyn))),
		ret(rt("json.stringifySubset", local("global", shape), str)),
	))
	runMethod.Static = true
	runMethod.Params = []hir.Param{{Name: "flag", Type: boolean}}
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: str, X: &hir.Expr{Kind: hir.This, Type: shape}, Name: "files"}, Y: local("files", str)}))
	ctor.Params = []hir.Param{{Name: "files", Type: str}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{
		{Name: "Global", Structural: true, Fields: []hir.Field{{Name: "files", Type: str}}, Ctor: ctor},
		{Name: "Probe", Methods: []*hir.Method{activate, runMethod}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	writer := files[names.Get("runtime.configJSON")+".clas.abap"]
	if !strings.Contains(writer, "own_child = p0->"+names.Get("view.extras")+"->get( own_row-k ).") {
		t.Fatal("native live shape extras refused by subset writer")
	}
	owner := names.Get("Probe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_probe IMPLEMENTATION.
METHOD check.
cl_abap_unit_assert=>assert_equals( act = %s=>%s( abap_false ) exp = `+"`"+`{"files":"/src/**","applyUnspecifiedRules":false}`+"`"+` ).
cl_abap_unit_assert=>assert_equals( act = %s=>%s( abap_true ) exp = `+"`"+`{"files":"/src/**","applyUnspecifiedRules":true}`+"`"+` ).
ENDMETHOD.
ENDCLASS.
`, owner, names.Get("member.run"), owner, names.Get("member.run"))
	if string(out) != "[\"{\\\"files\\\":\\\"/src/**\\\",\\\"applyUnspecifiedRules\\\":false}\",\"{\\\"files\\\":\\\"/src/**\\\",\\\"applyUnspecifiedRules\\\":true}\"]\n" {
		t.Fatalf("unexpected Node expectations: %s", out)
	}
	storageHostFixture(t, files, owner)
}
