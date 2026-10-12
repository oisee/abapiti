package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestClassArgsWideFieldConstructorABAP(t *testing.T) {
	cv, dyn := hir.T(hir.ClassValue), hir.T(hir.Dynamic)
	opt := hir.T(hir.Optional, str)
	wide := &hir.Class{Name: "Wide"}
	ctor := method("constructor", hir.T(hir.Void), hir.B())
	wide.Ctor = ctor
	for i := 0; i < 41; i++ {
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
	if strings.Count(args, "->"+names.Get("member.field0")+" = ") != 2 || strings.Count(args, "->"+names.Get("member.field40")+" = ") != 2 {
		t.Fatal("wide argument factory lost field-copy constructor semantics")
	}
	// Optional narrowing must still check presence, not assign an initial string.
	requireABI(t, args, "IS NOT BOUND.", "RAISE EXCEPTION TYPE "+names.Get("class.refusal")+".")
}
