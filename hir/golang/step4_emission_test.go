package golang

import (
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Refused live-field conversions still need a valid Go body, even when the
// fallback never consumes the dynamic value read from structural storage.
func TestRefusedLiveFieldGetterCompiles(t *testing.T) {
	row := &hir.Class{Name: "probe.ts.Row", Fields: []hir.Field{{Name: "factory", Type: hir.T(hir.ClassValue)}}}
	view := &hir.Method{Name: "view", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: hir.Ref(row.Name),
		Body: hir.B(ret(rt("dynamic.classView", local("value", hir.T(hir.Dynamic)), hir.Ref(row.Name))))}
	p := &hir.Program{Classes: []*hir.Class{row, {Name: "probe.ts", Methods: []*hir.Method{view}}}}
	n := hir.NewNames()
	main := `factory:=&classDescriptor{Name:str("native")};native:=` + n.Get("new."+row.Name) + `();native.` + n.Get("member.factory") + `=factory;if native.` + n.Get("view.read."+row.Name+".factory") + `()!=factory {panic("native field changed")};v:=` + entry("probe.ts", "view") + `(parseJSON(str("{}")));func(){defer func(){x:=recover();if fault,ok:=x.(trap);!ok || fault.Source!="unsupported live class field conversion" {panic(x)}}();v.` + n.Get("view.read."+row.Name+".factory") + `();panic("missing conversion refusal")}();fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

// Module-level functions use the static ABI even if their name is constructor.
func TestStaticFunctionNamedConstructorCompiles(t *testing.T) {
	ctor := &hir.Method{Name: "constructor", Static: true, Result: i32,
		Params: []hir.Param{{Name: "a", Type: i32}, {Name: "b", Type: i32}, {Name: "c", Type: i32}},
		Body:   hir.B(ret(binary("+", binary("+", local("a", i32), local("b", i32), i32), local("c", i32), i32)))}
	module := &hir.Class{Name: "tools/gogen/frontend.mjs", Methods: []*hir.Method{ctor}}
	p := &hir.Program{Classes: []*hir.Class{module}}
	if got := execute(t, p, `fmt.Println(`+entry(module.Name, "constructor")+`(1,2,3))`); got != "6\n" {
		t.Fatal(got)
	}
}
