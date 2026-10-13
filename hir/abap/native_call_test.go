package abap

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestNativeCall0Fixtures(t *testing.T) {
	out, err := exec.Command("node", "-e", `class B{value(){return 1}};class C extends B{value(){return 2}};console.log([new B,new C].reduce((n,v)=>n+v.value(),0)===3 && [1,undefined].map(v=>typeof v).join('')==='numberundefined');`).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	dyn := hir.T(hir.Dynamic)
	opt := hir.T(hir.Optional, i32)
	value := method("value", i32, ret(lit(1)))
	value.Virtual = true
	override := method("value", i32, ret(lit(2)))
	override.Virtual = true
	undef := method("undef", opt, ret(hir.L(opt, nil)))
	void := method("nothing", hir.T(hir.Void), hir.B())
	bridge := method("nothing_value", str, ret(hir.L(str, "bridge")))
	call0 := func(name string) *hir.Method {
		m := method(name, dyn, ret(rt("dynamic.call0", local("d", dyn), dyn, hir.L(str, name))))
		m.Static = true
		m.Params = []hir.Param{{Name: "d", Type: dyn}}
		return m
	}
	number := hir.T(hir.Number)
	base := hir.Ref("NativeBase")
	array := hir.T(hir.Array, base)
	loop := method("loop", boolean, hir.B(decl("a", array, newObj(array)), run(rt("array.push", local("a", array), i32, newObj(base))), run(rt("array.push", local("a", array), i32, newObj(hir.Ref("NativeChild")))), decl("sum", number, hir.L(number, 0.0)), &hir.Stmt{Kind: hir.ForEach, Name: "item", Type: base, X: local("a", array), Body: assign("sum", number, binary("+", local("sum", number), rt("dynamic.asNumber", rt("dynamic.call0", rt("dynamic.of", local("item", base), dyn), dyn, hir.L(str, "value")), number), number))}, ret(binary("==", local("sum", number), hir.L(number, 3.0), boolean))))
	loop.Static = true
	optional := hir.T(hir.Optional, i32)
	options := hir.T(hir.Array, optional)
	boxes := method("boxes", boolean, hir.B(decl("a", options, newObj(options)), run(rt("array.push", local("a", options), i32, lit(1))), run(rt("array.push", local("a", options), i32, hir.L(optional, nil))), decl("text", str, hir.L(str, "")), &hir.Stmt{Kind: hir.ForEach, Name: "item", Type: optional, X: local("a", options), Body: assign("text", str, rt("string.concat", local("text", str), str, rt("dynamic.typeofValue", rt("dynamic.of", local("item", optional), dyn), str)))}, ret(binary("==", local("text", str), hir.L(str, "numberundefined"), boolean))))
	boxes.Static = true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "NativeBase", Methods: []*hir.Method{value, undef, void, bridge}},
		{Name: "NativeChild", Super: "NativeBase", Methods: []*hir.Method{override}},
		{Name: "NativeUnsafe", Super: "NativeBase", PrototypeAccessors: true},
		{Name: "NativeCalls", Methods: []*hir.Method{call0("value"), call0("undef"), call0("nothing"), call0("missing"), loop, boxes}},
	}}
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	owner, d := names.Get("NativeCalls"), names.Get("runtime.dynamic")
	unit := fmt.Sprintf("CLASS ltcl_calls DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_calls IMPLEMENTATION.\nMETHOD check.\nDATA d TYPE REF TO %s.\nDATA v TYPE REF TO %s.\nDATA refused TYPE abap_bool.\nCREATE OBJECT d.\nd->tag = %s=>tag_ref.\n", d, d, d)
	unit += fmt.Sprintf("cl_abap_unit_assert=>assert_true( %s=>%s( ) ).\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\n", owner, names.Get("member.loop"), owner, names.Get("member.boxes"))
	for _, c := range []string{"NativeBase", "NativeChild"} {
		want := 1
		if c == "NativeChild" {
			want = 2
		}
		unit += fmt.Sprintf("d->oval = NEW %s( ).\nv = %s=>%s( d ).\ncl_abap_unit_assert=>assert_equals( act = v->nval exp = %d ).\nv = %s=>%s( d ).\ncl_abap_unit_assert=>assert_true( xsdbool( v IS NOT BOUND ) ).\nv = %s=>%s( d ).\ncl_abap_unit_assert=>assert_equals( act = v->sval exp = `bridge` ).\n", names.Get(c), owner, names.Get("member.value"), want, owner, names.Get("member.undef"), owner, names.Get("member.nothing"))
	}
	for _, c := range []string{"NativeUnsafe"} {
		unit += fmt.Sprintf("d->oval = NEW %s( ).\nCLEAR refused.\nTRY.\nv = %s=>%s( d ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get(c), owner, names.Get("member.value"), names.Get("class.refusal"))
	}
	unit += fmt.Sprintf("d->oval = NEW %s( ).\nCLEAR refused.\nTRY.\nv = %s=>%s( d ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.\nENDCLASS.\n", names.Get("NativeBase"), owner, names.Get("member.missing"), names.Get("class.refusal"))
	files[owner+".clas.testclasses.abap"] = unit
	storageHostFixture(t, files, owner)
}
