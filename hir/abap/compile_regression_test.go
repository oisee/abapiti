package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestClassValueSignatureEnablesDescriptorInterface(t *testing.T) {
	cv := hir.T(hir.ClassValue)
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "Early"},
		{Name: "SignatureOnly", Fields: []hir.Field{{Name: "constructorValue", Type: hir.T(hir.Optional, cv)}}},
	}}
	files, names := abiSource(t, p)
	iface := files[names.Get("runtime.described")+".intf.abap"]
	requireABI(t, iface, "METHODS "+names.Get("builtin.classOf"), "METHODS "+names.Get("builtin.nativeConstructor"))
	for _, c := range p.Classes {
		requireABI(t, files[names.Get(c.Name)+".clas.abap"], "INTERFACES "+names.Get("runtime.described"))
	}
}

func TestJSONSubsetEnablesDescriptorInterfaceBeforeClasses(t *testing.T) {
	d := hir.T(hir.Dynamic)
	m := method("roundtrip", str, ret(rt("json.stringifySubset", rt("json.parseSubset", local("input", str), d), str)))
	m.Static, m.Params = true, []hir.Param{{Name: "input", Type: str}}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Early"}, {Name: "JSONProbe", Methods: []*hir.Method{m}}}}
	files, names := abiSource(t, p)
	iface := files[names.Get("runtime.described")+".intf.abap"]
	requireABI(t, iface, "METHODS "+names.Get("builtin.classOf"), "METHODS "+names.Get("builtin.nativeConstructor"))
	for _, owner := range []string{"Early", "JSONProbe", "builtin.Error"} {
		src := files[names.Get(owner)+".clas.abap"]
		requireABI(t, src, "INTERFACES "+names.Get("runtime.described"))
		for _, member := range []string{"builtin.classOf", "builtin.nativeConstructor"} {
			requireABI(t, src, "METHODS "+names.Get(member), "METHOD "+names.Get("runtime.described")+"~"+names.Get(member))
		}
	}
}

func TestLiveBoxUsesCheckedDynamicCast(t *testing.T) {
	d := hir.T(hir.Dynamic)
	array := hir.T(hir.Array, hir.Ref(hir.RootObject))
	view := rt("dynamic.asRef", local("input", d), array)
	m := method("box", d, ret(rt("dynamic.of", view, d)))
	m.Static, m.Params = true, []hir.Param{{Name: "input", Type: d}}
	files, names := abiSource(t, &hir.Program{Classes: []*hir.Class{{Name: "LiveBoxProbe", Methods: []*hir.Method{m}}}})
	src := files[names.Get("runtime."+array.String())+".clas.abap"]
	requireABI(t, src, "TRY.", "CATCH cx_sy_move_cast_error.")
	if strings.Contains(src, "IS INSTANCE OF "+names.Get("runtime.dynamic")) {
		t.Fatal("live-box type test prevents the config JSON v702 dependency from parsing", src)
	}
}

func TestOptionalRootClassOfUsesCheckedDescriptor(t *testing.T) {
	cv := hir.T(hir.ClassValue)
	root := hir.T(hir.Optional, hir.Ref(hir.RootObject))
	m := method("type", cv, ret(rt("object.classOf", local("input", root), cv)))
	m.Params = []hir.Param{{Name: "input", Type: root}}
	files, names := abiSource(t, &hir.Program{Classes: []*hir.Class{{Name: "Caller", Methods: []*hir.Method{m}}}})
	src := files[names.Get("Caller")+".clas.abap"]
	requireABI(t, src, "IS BOUND.", " ?= ", "CATCH cx_sy_move_cast_error.")
	// Optional root storage is still erased: only the checked interface view
	// may receive the native-constructor method call (absence remains absent).
	if strings.Count(src, "->"+names.Get("builtin.nativeConstructor")+"( )") != 2 {
		t.Fatal("missing checked optional-root descriptor call", src)
	}
}
