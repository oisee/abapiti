package golang

import (
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Live class-descriptor fields retain their identity and reject other values.
func TestLiveClassDescriptorFieldConversion(t *testing.T) {
	row := &hir.Class{Name: "probe.ts.Row", Fields: []hir.Field{{Name: "factory", Type: hir.T(hir.ClassValue)}}}
	view := &hir.Method{Name: "view", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: hir.Ref(row.Name),
		Body: hir.B(ret(rt("dynamic.classView", local("value", hir.T(hir.Dynamic)), hir.Ref(row.Name))))}
	p := &hir.Program{Classes: []*hir.Class{row, {Name: "probe.ts", Methods: []*hir.Method{view}}}}
	n := hir.NewNames()
	main := `factory:=&classDescriptor{Name:str("native")};native:=` + n.Get("new."+row.Name) + `();native.` + n.Get("member.factory") + `=factory;if native.` + n.Get("view.read."+row.Name+".factory") + `()!=factory {panic("native field changed")};d:=parseJSON(str("{}"));d.put(str("factory"),box(factory));v:=` + entry("probe.ts", "view") + `(d);if v.` + n.Get("view.read."+row.Name+".factory") + `()!=factory {panic("descriptor identity changed")};bad:=` + entry("probe.ts", "view") + `(parseJSON(str("{\"factory\":{}}")));func(){defer func(){if x:=recover();x!=nil {if _,ok:=x.(rangeFault);!ok {panic(x)}} else {panic("missing fault")}}();bad.` + n.Get("view.read."+row.Name+".factory") + `()}();fmt.Println("ok")`
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
