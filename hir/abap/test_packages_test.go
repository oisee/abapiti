package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestTestPackageDispatch(t *testing.T) {
	cv, dyn := hir.T(hir.ClassValue), hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, dyn)
	make := method("make", hir.Ref(hir.RootObject), ret(rt("classvalue.newArgs", local("ctor", cv), hir.Ref(hir.RootObject), local("args", arr))))
	make.Static = true
	make.Params = []hir.Param{{Name: "ctor", Type: cv}, {Name: "args", Type: arr}}
	remove := method("remove", hir.T(hir.Bool), ret(rt("dynamic.delete", local("obj", dyn), hir.T(hir.Bool), local("key", str))))
	remove.Static = true
	remove.Params = []hir.Param{{Name: "obj", Type: dyn}, {Name: "key", Type: str}}
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "src/core.ts.Core"},
		{Name: "test/body.ts.TestBody"},
		{Name: "closure.1", Node: hir.Node{Source: "test/body.ts:5:1"}},
		{Name: "src/caller.ts.Caller", Methods: []*hir.Method{make, remove}},
	}}
	files, names, err := EmitTestPackageNamed(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	factory := files[names.Get("runtime.classvalue.factory")+".clas.abap"]
	deletion := files[names.Get("runtime.nativeDelete")+".clas.abap"]
	for _, id := range []string{"test/body.ts.TestBody", "closure.1"} {
		name := names.Get(id)
		if strings.Contains(factory, "TYPE "+name) || strings.Contains(deletion, "OF "+name) {
			t.Fatal("reverse core/test dependency", name)
		}
		if !strings.Contains(files["zcl_lnt_test_factory.clas.abap"], "TYPE "+name) {
			t.Fatal("missing test constructor", name)
		}
	}
	for _, file := range []string{"zif_lnt_runtime_extension.intf.abap", "zcl_lnt_test_extension.clas.abap", "zcl_lnt_test_delete.clas.abap"} {
		if files[file] == "" {
			t.Fatal("missing", file)
		}
	}
	if !strings.Contains(factory, "extension->new_args") || !strings.Contains(deletion, "extension->delete") {
		t.Fatal("extension dispatch missing")
	}
	for file, source := range files {
		for _, line := range strings.Split(source, "\n") {
			if len(line) > 255 {
				t.Fatal("line envelope", file)
			}
		}
	}
}
