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

// A typed reference array viewed as any[] (abaplint's testRuleWithVariableConfig
// receives `tests: any` and calls forEach): reference storage keeps the
// original row objects, so element reads box them like Go's box().
func erasedElementFixture() *hir.Program {
	dyn, row := hir.T(hir.Dynamic), hir.Ref("ErasedRow")
	rows, anyRows := hir.T(hir.Array, row), hir.T(hir.Array, dyn)
	field := func(x *hir.Expr) *hir.Expr { return &hir.Expr{Kind: hir.FieldGet, X: x, Name: "d", Type: str} }
	text := func(x *hir.Expr) *hir.Expr { return field(rt("dynamic.asRef", x, row)) }
	optDyn := hir.T(hir.Optional, dyn)
	cat := func(a, b *hir.Expr) *hir.Expr { return rt("string.concat", a, str, b) }
	body := hir.B(decl("rows", rows, newObj(rows)))
	for i, v := range []string{"a", "b", "c"} {
		name := fmt.Sprintf("r%d", i)
		body.List = append(body.List,
			decl(name, row, newObj(row)),
			&hir.Stmt{Kind: hir.Assign, X: field(local(name, row)), Y: hir.L(str, v)},
			run(rt("array.push", local("rows", rows), i32, local(name, row))))
	}
	body.List = append(body.List,
		decl("boxed", dyn, rt("dynamic.of", local("rows", rows), dyn)),
		decl("view", anyRows, rt("dynamic.asRef", local("boxed", dyn), anyRows)),
		decl("s", str, hir.L(str, "")),
		&hir.Stmt{Kind: hir.ForEach, Name: "e", Type: dyn, X: local("view", anyRows), Body: hir.B(
			assign("s", str, cat(local("s", str), text(local("e", dyn)))))},
		assign("s", str, cat(local("s", str), text(&hir.Expr{Kind: hir.Narrow, Type: dyn, X: rt("array.get", local("view", anyRows), optDyn, lit(1))}))),
		assign("s", str, cat(local("s", str), text(&hir.Expr{Kind: hir.IndexGet, X: local("view", anyRows), Y: lit(2), Type: dyn}))),
		// A row that is already a tagged value is read unchanged.
		decl("mixed", anyRows, newObj(anyRows)),
		run(rt("array.push", local("mixed", anyRows), i32, rt("dynamic.of", hir.L(str, "x"), dyn))),
		assign("s", str, cat(local("s", str), rt("dynamic.asString", &hir.Expr{Kind: hir.Narrow, Type: dyn, X: rt("array.get", local("mixed", anyRows), optDyn, lit(0))}, str))),
		ret(local("s", str)),
	)
	m := method("read", str, body)
	m.Static = true
	return &hir.Program{Classes: []*hir.Class{
		{Name: "ErasedRow", Fields: []hir.Field{{Name: "d", Type: str}}},
		{Name: "ErasedRows", Methods: []*hir.Method{m}},
	}}
}

func TestErasedArrayElementBoxing(t *testing.T) {
	if _, err := exec.LookPath("node"); err == nil {
		const oracle = `const rows=[{d:"a"},{d:"b"},{d:"c"}];const view=rows;let s="";view.forEach(e=>s+=e.d);s+=view[1].d+view[2].d+["x"][0];console.log(s)`
		want, err := exec.Command("node", "-e", oracle).CombinedOutput()
		if err != nil || string(want) != "abcbcx\n" {
			t.Fatalf("Node oracle: %s %v", want, err)
		}
	}
	p := erasedElementFixture()
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("ErasedRows")
	src := files[owner+".clas.abap"]
	// Three erased reads (loop row, get, index) box when the slot holds no tag.
	if got := strings.Count(src, "=>tag_ref."); got < 3 {
		t.Fatalf("expected boxing fallback for each erased read, found %d:\n%s", got, src)
	}
	if strings.Count(src, " IS INSTANCE OF "+names.Get("runtime.dynamic")+".") < 3 {
		t.Fatalf("expected tagged rows to be read unchanged:\n%s", src)
	}
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_erased DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_erased IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = `abcbcx` ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.read"))
	writeTestOut(t, files)
}

// The frontend bridge for an erased `any` slot selects the live structural
// view (abaplint's IRule.setConfig(conf: any) with object-literal configs).
// The ABAP slot forwarder must use the same conversion, not a nominal cast.
func TestSlotForwarderUsesBridgeClassView(t *testing.T) {
	dyn, conf := hir.T(hir.Dynamic), hir.Ref("FwdConf")
	slot := method("setConfig", hir.T(hir.Void), nil)
	slot.Params, slot.Virtual, slot.Abstract = []hir.Param{{Name: "conf", Type: dyn}}, true, true
	impl := method("setConfig_instantiated_FwdRule", hir.T(hir.Void), hir.B(
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("FwdRule")}, Name: "conf", Type: conf}, Y: local("conf", conf)}))
	impl.Params, impl.Virtual = []hir.Param{{Name: "conf", Type: conf}}, true
	bridge := method("setConfig", hir.T(hir.Void), hir.B(run(&hir.Expr{Kind: hir.VirtualCall, Name: impl.Name, Type: hir.T(hir.Void),
		X: &hir.Expr{Kind: hir.This, Type: hir.Ref("FwdRule")}, Args: []*hir.Expr{rt("dynamic.classView", local("conf", dyn), conf)}})))
	bridge.Params, bridge.Virtual = []hir.Param{{Name: "conf", Type: dyn}}, true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "FwdConf", Fields: []hir.Field{{Name: "flag", Type: boolean}}},
		{Name: "FwdBase", Abstract: true, Methods: []*hir.Method{slot}},
		{Name: "FwdRule", Super: "FwdBase", Fields: []hir.Field{{Name: "conf", Type: conf}}, Methods: []*hir.Method{bridge, impl}},
	}}
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	src := files[names.Get("FwdRule")+".clas.abap"]
	start := strings.Index(src, "METHOD "+names.Get("member.setConfig")+".")
	if start < 0 {
		t.Fatalf("no slot forwarder:\n%s", src)
	}
	forwarder := src[start:]
	forwarder = forwarder[:strings.Index(forwarder, "ENDMETHOD.")]
	if strings.Contains(forwarder, "as_ref( )") || !strings.Contains(forwarder, "view_source = ") || !strings.Contains(forwarder, names.Get("view.refusal")) {
		t.Fatalf("slot forwarder does not use the live class view:\n%s", forwarder)
	}
}

func writeTestOut(t *testing.T, files map[string]string) {
	out := os.Getenv("ABAPITI_TEST_OUT")
	if out == "" {
		return
	}
	out = filepath.Join(out, t.Name())
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for name, source := range files {
		source, err := Format(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
