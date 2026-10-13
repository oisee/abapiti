package abap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// This fixture crosses multiple dispatcher leaves and checks every constructor
// through emitted ABAP Unit, including missing optional arguments and surplus
// arguments. Export it with ABAPITI_TEST_OUT for the pinned OSG unit runner.
func TestClassFactorySplitFixtures(t *testing.T) {
	cv, dyn := hir.T(hir.ClassValue), hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, dyn)
	opt := hir.T(hir.Optional, str)
	p := &hir.Program{}
	for i := 0; i < 180; i++ {
		name := fmt.Sprintf("Factory%d", i)
		ctor := method("constructor", hir.T(hir.Void), hir.B(
			&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: "first", Type: str, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}}, Y: local("first", str)},
			&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: "second", Type: opt, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}}, Y: local("second", opt)},
		))
		ctor.Params = []hir.Param{{Name: "first", Type: str}, {Name: "second", Type: opt}}
		p.Classes = append(p.Classes, &hir.Class{Name: name, Ctor: ctor, Fields: []hir.Field{{Name: "first", Type: str}, {Name: "second", Type: opt}}})
	}
	for i := 0; i < 80; i++ {
		p.Classes = append(p.Classes, &hir.Class{Name: fmt.Sprintf("Empty%d", i), Scoped: true})
	}
	make := method("make", hir.Ref(hir.RootObject), ret(rt("classvalue.newArgs", local("ctor", cv), hir.Ref(hir.RootObject), local("args", arr))))
	make.Static = true
	make.Params = []hir.Param{{Name: "ctor", Type: cv}, {Name: "args", Type: arr}}
	p.Classes = append(p.Classes, &hir.Class{Name: "FactoryCaller", Methods: []*hir.Method{make}})
	files, names := abiSource(t, p)
	factory := names.Get("runtime.classvalue.factory")
	src := files[factory+".clas.abap"]
	lines := 0
	methods := 0
	for _, line := range strings.Split(src, "\n") {
		word := strings.TrimSpace(line)
		if strings.HasPrefix(word, "METHOD ") {
			lines = 0
			methods++
		}
		lines++
		if word == "ENDMETHOD." && lines > 1000 {
			t.Fatalf("factory method has %d lines", lines)
		}
		if strings.HasPrefix(word, "METHOD ") && len(strings.TrimSuffix(strings.TrimPrefix(word, "METHOD "), ".")) > 30 {
			t.Fatal("overlong method name")
		}
	}
	if methods < 10 {
		t.Fatal("factory did not split")
	}
	again, _, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	if src != again[factory+".clas.abap"] {
		t.Fatal("nondeterministic factory")
	}

	// Get runtime type spellings from the public factory signature, rather than
	// duplicating the emitter's array/optional naming scheme.
	dynName := names.Get("runtime.dynamic")
	arrayName := regexp.MustCompile(`p1 TYPE REF TO (\w+)`).FindStringSubmatch(src)[1]
	staticArray := regexp.MustCompile(`static_names TYPE REF TO (\w+)`).FindStringSubmatch(files[names.Get("runtime.classvalue")+".clas.abap"])[1]
	var defs, impl strings.Builder
	descriptor := names.Get("runtime.classvalue")
	for i, c := range p.Classes[:180] {
		test := fmt.Sprintf("route%d", i)
		fmt.Fprintf(&defs, "METHODS %s FOR TESTING.\n", test)
		fmt.Fprintf(&impl, "METHOD %s.\nDATA parent TYPE REF TO %s.\nDATA statics TYPE REF TO %s.\nDATA descriptor TYPE REF TO %s.\nDATA args TYPE REF TO %s.\nDATA box TYPE REF TO %s.\nDATA second TYPE REF TO %s.\nDATA raw TYPE REF TO object.\nDATA instance TYPE REF TO %s.\n", test, descriptor, staticArray, descriptor, arrayName, dynName, dynName, names.Get(c.Name))
		// The descriptor carries the concrete array type even when it is absent.
		fmt.Fprintf(&impl, "CREATE OBJECT descriptor EXPORTING name = `fixture` emitted = `%s` parent = parent static_names = statics.\nCREATE OBJECT args.\nCREATE OBJECT box.\nbox->tag = %s=>tag_string.\nbox->sval = `first`.\nAPPEND box TO args->items.\n", names.Get(c.Name), dynName)
		if i%2 == 0 {
			fmt.Fprintf(&impl, "CREATE OBJECT second.\nsecond->tag = %s=>tag_string.\nsecond->sval = `second`.\nAPPEND second TO args->items.\nAPPEND box TO args->items.\n", dynName)
		}
		fmt.Fprintf(&impl, "raw = %s=>new_args( p0 = descriptor p1 = args ).\ninstance ?= raw.\ncl_abap_unit_assert=>assert_equals( act = instance->%s exp = `first` ).\n", factory, names.Get("member.first"))
		if i%2 == 0 {
			fmt.Fprintf(&impl, "cl_abap_unit_assert=>assert_true( instance->%s->has ).\ncl_abap_unit_assert=>assert_equals( act = instance->%s->value exp = `second` ).\n", names.Get("member.second"), names.Get("member.second"))
		} else {
			fmt.Fprintf(&impl, "cl_abap_unit_assert=>assert_false( xsdbool( instance->%s IS BOUND ) ).\n", names.Get("member.second"))
		}
		impl.WriteString("ENDMETHOD.\n")
	}
	for i, c := range p.Classes[180:260] {
		test := fmt.Sprintf("empty%d", i)
		fmt.Fprintf(&defs, "METHODS %s FOR TESTING.\n", test)
		fmt.Fprintf(&impl, "METHOD %s.\nDATA parent TYPE REF TO %s.\nDATA statics TYPE REF TO %s.\nDATA descriptor TYPE REF TO %s.\nDATA args TYPE REF TO %s.\nDATA raw TYPE REF TO object.\nDATA instance TYPE REF TO %s.\n", test, descriptor, staticArray, descriptor, arrayName, names.Get(c.Name))
		fmt.Fprintf(&impl, "CREATE OBJECT descriptor EXPORTING name = `fixture` emitted = `%s` parent = parent static_names = statics.\ndescriptor->lexical = abap_true.\n", names.Get(c.Name))
		for _, call := range []string{"new( p0 = descriptor )", "new_args( p0 = descriptor p1 = args )"} {
			fmt.Fprintf(&impl, "raw = %s=>%s.\ninstance ?= raw.\ncl_abap_unit_assert=>assert_true( xsdbool( instance IS BOUND ) ).\ncl_abap_unit_assert=>assert_true( xsdbool( instance->%s( ) = descriptor ) ).\n", factory, call, names.Get("builtin.classOf"))
		}
		impl.WriteString("ENDMETHOD.\n")
	}
	defs.WriteString("METHODS refusals FOR TESTING.\n")
	fmt.Fprintf(&impl, "METHOD refusals.\nDATA parent TYPE REF TO %s.\nDATA statics TYPE REF TO %s.\nDATA descriptor TYPE REF TO %s.\nDATA args TYPE REF TO %s.\nDATA box TYPE REF TO %s.\nDATA second TYPE REF TO %s.\nDATA raw TYPE REF TO object.\nDATA trapped TYPE abap_bool.\n", descriptor, staticArray, descriptor, arrayName, dynName, dynName)
	fmt.Fprintf(&impl, "CREATE OBJECT descriptor EXPORTING name = `fixture` emitted = `%s` parent = parent static_names = statics.\nCREATE OBJECT args.\n", names.Get("Factory0"))
	for _, setup := range []string{"", "descriptor->emitted = `UNKNOWN`.\n", "CLEAR descriptor.\n"} {
		impl.WriteString(setup + "trapped = abap_false.\nTRY.\n")
		fmt.Fprintf(&impl, "raw = %s=>new_args( p0 = descriptor p1 = args ).\nCATCH %s.\ntrapped = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( trapped ).\n", factory, names.Get("class.refusal"))
	}
	impl.WriteString("ENDMETHOD.\n")
	unit := "CLASS ltcl_factory DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\n" + defs.String() + "ENDCLASS.\nCLASS ltcl_factory IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
	out := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out = filepath.Join(base, t.Name())
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	files[names.Get("FactoryCaller")+".clas.testclasses.abap"] = unit
	for name, code := range files {
		code, err = Format(code)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(out, name), []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
