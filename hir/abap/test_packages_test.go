package abap

import (
	"fmt"
	"os"
	"path/filepath"
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
	if out := fixtureOutputDir(t); out != "" {
		files["zcl_lnt_dispatch_probe.clas.abap"] = "CLASS zcl_lnt_dispatch_probe DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS zcl_lnt_dispatch_probe IMPLEMENTATION.\nENDCLASS.\n"
		factoryName := names.Get("runtime.classvalue.factory")
		cvName := names.Get("runtime.classvalue")
		statics := names.Get("runtime." + hir.T(hir.Array, str).String())
		probe := fmt.Sprintf(`CLASS ltcl_probe DEFINITION FOR TESTING RISK LEVEL HARMLESS DURATION SHORT.
PRIVATE SECTION.
METHODS dispatch FOR TESTING.
ENDCLASS.
CLASS ltcl_probe IMPLEMENTATION.
METHOD dispatch.
DATA descriptor TYPE REF TO %s.
DATA parent TYPE REF TO %s.
DATA statics TYPE REF TO %s.
DATA raw TYPE REF TO object.
DATA core TYPE REF TO %s.
DATA body TYPE REF TO %s.
CREATE OBJECT descriptor EXPORTING name = 'fixture' emitted = '%s' parent = parent static_names = statics.
raw = %s=>new( descriptor ).
core ?= raw.
cl_abap_unit_assert=>assert_true( xsdbool( core IS BOUND ) ).
%s=>extension = NEW zcl_lnt_test_extension( ).
raw = %s=>new( descriptor ).
core ?= raw.
cl_abap_unit_assert=>assert_true( xsdbool( core IS BOUND ) ).
descriptor->emitted = '%s'.
raw = %s=>new( descriptor ).
body ?= raw.
cl_abap_unit_assert=>assert_true( xsdbool( body IS BOUND ) ).
cl_abap_unit_assert=>assert_true( %s=>run( p0 = raw key = 'absent' ) ).
raw = %s=>new_args( p0 = descriptor p1 = VALUE #( ) ).
body ?= raw.
cl_abap_unit_assert=>assert_true( xsdbool( body IS BOUND ) ).
ENDMETHOD.
ENDCLASS.
`, cvName, cvName, statics, names.Get("src/core.ts.Core"), names.Get("test/body.ts.TestBody"), names.Get("src/core.ts.Core"), factoryName, factoryName, factoryName, names.Get("test/body.ts.TestBody"), factoryName, names.Get("runtime.nativeDelete"), factoryName)
		formatted, err := Format(probe)
		if err != nil {
			t.Fatal(err)
		}
		files["zcl_lnt_dispatch_probe.clas.testclasses.abap"] = formatted
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for file, source := range files {
			if err := os.WriteFile(filepath.Join(out, file), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for file, source := range files {
		for _, line := range strings.Split(source, "\n") {
			if len(line) > 255 {
				t.Fatal("line envelope", file)
			}
		}
	}
}
