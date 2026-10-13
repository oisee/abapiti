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

func TestClassArgsWideFieldConstructorABAP(t *testing.T) { wideFactoryFixture(t, 41) }

func TestClassArgsHugeFieldConstructorABAP(t *testing.T) { wideFactoryFixture(t, 317) }

func wideFactoryFixture(t *testing.T, width int) {
	t.Helper()
	cv, dyn := hir.T(hir.ClassValue), hir.T(hir.Dynamic)
	opt := hir.T(hir.Optional, str)
	wide := &hir.Class{Name: "Wide"}
	ctor := method("constructor", hir.T(hir.Void), hir.B())
	wide.Ctor = ctor
	for i := 0; i < width; i++ {
		name := fmt.Sprintf("field%d", i)
		fieldType := opt
		value := local(name, opt)
		if i == 0 {
			fieldType = str
			value = &hir.Expr{Kind: hir.Narrow, Type: str, X: value}
		}
		wide.Fields = append(wide.Fields, hir.Field{Name: name, Type: fieldType})
		ctor.Params = append(ctor.Params, hir.Param{Name: name, Type: opt})
		ctor.Body.List = append(ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: fieldType, Name: name, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Wide")}}, Y: value})
	}
	m := method("make", hir.Ref(hir.RootObject), ret(rt("classvalue.newArgs", local("ctor", cv), hir.Ref(hir.RootObject), local("args", hir.T(hir.Array, dyn)))))
	m.Params = []hir.Param{{Name: "ctor", Type: cv}, {Name: "args", Type: hir.T(hir.Array, dyn)}}
	files, names := abiSource(t, &hir.Program{Classes: []*hir.Class{wide, {Name: "InheritedWide", Super: "Wide"}, {Name: "Caller", Methods: []*hir.Method{m}}}})
	src := files[names.Get("runtime.classvalue.factory")+".clas.abap"]
	args := src[strings.Index(src, "METHOD new_args."):]
	for _, c := range []string{"Wide", "InheritedWide"} {
		requireABI(t, args, "CREATE OBJECT result TYPE "+names.Get(c)+".")
		if strings.Contains(args, "CREATE OBJECT result TYPE "+names.Get(c)+" EXPORTING") {
			t.Fatal("wide shape has no ABAP constructor to receive EXPORTING arguments")
		}
	}
	if strings.Count(args, "->"+names.Get("member.field0")+" = ") != 2 || strings.Count(args, "->"+names.Get(fmt.Sprintf("member.field%d", width-1))+" = ") != 2 {
		t.Fatal("wide argument factory lost field-copy constructor semantics")
	}
	lines := 0
	for _, line := range strings.Split(src, "\n") {
		word := strings.TrimSpace(line)
		if strings.HasPrefix(word, "METHOD ") {
			lines = 0
		}
		lines++
		if word == "ENDMETHOD." && lines > 1000 {
			t.Fatalf("wide factory method has %d lines", lines)
		}
	}
	// Optional narrowing must still check presence, not assign an initial string.
	requireABI(t, args, "IS NOT BOUND.", "RAISE EXCEPTION TYPE "+names.Get("class.refusal")+".")
	descriptor, factory := names.Get("runtime.classvalue"), names.Get("runtime.classvalue.factory")
	array := regexp.MustCompile(`p1 TYPE REF TO (\w+)`).FindStringSubmatch(src)[1]
	statics := regexp.MustCompile(`static_names TYPE REF TO (\w+)`).FindStringSubmatch(files[descriptor+".clas.abap"])[1]
	var defs, impl strings.Builder
	for _, class := range []string{"Wide", "InheritedWide"} {
		for _, mode := range []string{"full", "missing", "bad"} {
			test := strings.ToLower(class) + "_" + mode
			fmt.Fprintf(&defs, "METHODS %s FOR TESTING.\n", test)
			fmt.Fprintf(&impl, "METHOD %s.\nDATA parent TYPE REF TO %s.\nDATA statics TYPE REF TO %s.\nDATA descriptor TYPE REF TO %s.\nDATA args TYPE REF TO %s.\nDATA box TYPE REF TO %s.\nDATA bad TYPE REF TO %s.\nDATA raw TYPE REF TO object.\nDATA instance TYPE REF TO %s.\nDATA trapped TYPE abap_bool.\n", test, descriptor, statics, descriptor, array, names.Get("runtime.dynamic"), names.Get("runtime.dynamic"), names.Get(class))
			fmt.Fprintf(&impl, "CREATE OBJECT descriptor EXPORTING name = `fixture` emitted = `%s` parent = parent static_names = statics.\nCREATE OBJECT args.\nCREATE OBJECT box.\nbox->tag = %s=>tag_string.\nbox->sval = `value`.\n", names.Get(class), names.Get("runtime.dynamic"))
			if mode != "missing" {
				fmt.Fprintf(&impl, "DO %d TIMES.\nCREATE OBJECT box.\nbox->tag = %s=>tag_string.\nbox->sval = |slot{ sy-index }|.\nAPPEND box TO args->items.\nENDDO.\n", width-1, names.Get("runtime.dynamic"))
				if mode == "bad" {
					fmt.Fprintf(&impl, "CREATE OBJECT bad.\nbad->tag = %s=>tag_boolean.\nAPPEND bad TO args->items.\n", names.Get("runtime.dynamic"))
				} else {
					fmt.Fprintf(&impl, "CREATE OBJECT box.\nbox->tag = %s=>tag_string.\nbox->sval = `last`.\nAPPEND box TO args->items.\n", names.Get("runtime.dynamic"))
				}
			}
			if mode != "full" {
				impl.WriteString("TRY.\n")
			}
			fmt.Fprintf(&impl, "raw = %s=>new_args( p0 = descriptor p1 = args ).\n", factory)
			if mode == "full" {
				fmt.Fprintf(&impl, "instance ?= raw.\ncl_abap_unit_assert=>assert_equals( act = instance->%s exp = `slot1` ).\ncl_abap_unit_assert=>assert_equals( act = instance->%s->value exp = `last` ).\n", names.Get("member.field0"), names.Get(fmt.Sprintf("member.field%d", width-1)))
				fmt.Fprintf(&impl, "cl_abap_unit_assert=>assert_equals( act = instance->%s->value exp = `slot25` ).\n", names.Get("member.field24"))
			} else {
				trap := names.Get("class.refusal")
				if mode == "bad" {
					trap = "cx_sy_move_cast_error"
				}
				fmt.Fprintf(&impl, "CATCH %s.\ntrapped = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( trapped ).\n", trap)
			}
			impl.WriteString("ENDMETHOD.\n")
		}
	}
	files[names.Get("Caller")+".clas.testclasses.abap"] = "CLASS ltcl_wide DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\n" + defs.String() + "ENDCLASS.\nCLASS ltcl_wide IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
	out := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out = filepath.Join(base, t.Name())
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for name, source := range files {
		source, err := Format(source)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}

}
