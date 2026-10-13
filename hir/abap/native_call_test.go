package abap

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestNativeCall0Fixtures(t *testing.T) {
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
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "NativeBase", Methods: []*hir.Method{value, undef, void, bridge}},
		{Name: "NativeChild", Super: "NativeBase", Methods: []*hir.Method{override}},
		{Name: "NativeUnsafe", Super: "NativeBase", PrototypeAccessors: true},
		{Name: "NativeCalls", Methods: []*hir.Method{call0("value"), call0("undef"), call0("nothing"), call0("missing")}},
	}}
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	owner, d := names.Get("NativeCalls"), names.Get("runtime.dynamic")
	unit := fmt.Sprintf("CLASS ltcl_calls DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_calls IMPLEMENTATION.\nMETHOD check.\nDATA d TYPE REF TO %s.\nDATA v TYPE REF TO %s.\nDATA refused TYPE abap_bool.\nCREATE OBJECT d.\nd->tag = %s=>tag_ref.\n", d, d, d)
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
