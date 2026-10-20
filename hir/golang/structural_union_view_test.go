package golang

import (
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A class that already implements one structural-union alternative can carry
// callable members that the data-only guard cannot inspect. Boxing and reading
// it through the union field should preserve the exact typed instance before
// attempting external-record membership checks.
func TestLiveStructuralUnionReusesExactTypedMemberBeforeShapeChecks(t *testing.T) {
	p := typedStructuralUnionProgram()
	helper, container, module := p.Classes[1], p.Classes[3], p.Classes[4]
	names := hir.NewNames()
	main := `typed:=&` + names.Get("struct."+helper.Name) + `{};record:=parseJSON(str("{}"));record.put(str("helpers"),box(typed));view:=` + entry(module.Name, "view") + `(record);if view.` + names.Get("view.read."+container.Name+".helpers") + `()!=typed {panic("exact typed union member identity changed")};`
	main += `badHelpers:=parseJSON(str("{}"));badRecord:=parseJSON(str("{}"));badRecord.put(str("helpers"),badHelpers);badView:=` + entry(module.Name, "view") + `(badRecord);func(){defer func(){if x:=recover();x!=nil {if _,ok:=x.(rangeFault);!ok {panic(x)}} else {panic("invalid external union member was accepted")}}();badView.` + names.Get("view.read."+container.Name+".helpers") + `()}();fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func typedStructuralUnionProgram() *hir.Program {
	callback := &hir.Class{Name: "probe.ts.Callback", Structural: true}
	callbackType := hir.Ref(callback.Name)
	helper := &hir.Class{Name: "probe.ts.ConstantHelpers", Structural: true, Implements: []string{"probe.ts.Helpers"}, Fields: []hir.Field{
		{Name: "js", Type: hir.T(hir.Optional, hir.Type{Kind: hir.Bool})},
		{Name: "type", Type: callbackType},
		{Name: "literal", Type: callbackType},
	}}
	jsDecl := &hir.Class{Name: "probe.ts.JsDeclarations", Structural: true, Implements: []string{"probe.ts.Helpers"}, Fields: []hir.Field{
		{Name: "js", Type: hir.Type{Kind: hir.Bool}},
		{Name: "literal", Type: callbackType},
	}}
	helperFields := []hir.StructuralField{
		{Name: "js", Type: hir.T(hir.Optional, hir.Type{Kind: hir.Bool}), Optional: true, Domains: []hir.StructuralDomain{{Kind: hir.Bool, Exact: true, Literal: "false"}}},
		{Name: "type", Type: callbackType},
		{Name: "literal", Type: callbackType},
	}
	jsFields := []hir.StructuralField{
		{Name: "js", Type: hir.Type{Kind: hir.Bool}, Domains: []hir.StructuralDomain{{Kind: hir.Bool, Exact: true, Literal: "true"}}},
		{Name: "literal", Type: callbackType},
	}
	helpers := &hir.Interface{Name: "probe.ts.Helpers", StructuralMembers: []hir.StructuralMember{
		{Type: hir.Ref(helper.Name), Fields: helperFields},
		{Type: hir.Ref(jsDecl.Name), Fields: jsFields},
	}}
	container := &hir.Class{Name: "probe.ts.Container", Structural: true, Fields: []hir.Field{{Name: "helpers", Type: hir.Type{Kind: hir.InterfaceRef, Name: helpers.Name}}}}
	view := &hir.Method{Name: "view", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: hir.Ref(container.Name),
		Body: hir.B(ret(rt("dynamic.classView", local("value", hir.T(hir.Dynamic)), hir.Ref(container.Name))))}
	module := &hir.Class{Name: "probe.ts", Methods: []*hir.Method{view}}
	return &hir.Program{Classes: []*hir.Class{callback, helper, jsDecl, container, module}, Interfaces: []*hir.Interface{helpers}}
}

// A native carrier belongs only to its concrete source member, even when a
// mutation makes its discriminant resemble another alternative.
func TestLiveStructuralUnionMutatedTypedMemberRangeFault(t *testing.T) {
	p := typedStructuralUnionProgram()
	n := hir.NewNames()
	main := `typed:=&` + n.Get("struct.probe.ts.ConstantHelpers") + `{};typed.` + n.Get("member.js") + `=present(true);record:=parseJSON(str("{}"));record.put(str("helpers"),box(typed));view:=` + entry("probe.ts", "view") + `(record);defer func(){x:=recover();if _,ok:=x.(rangeFault);!ok {panic(fmt.Sprintf("expected rangeFault, got %T: %v",x,x))};fmt.Println("ok")}();view.` + n.Get("view.read.probe.ts.Container.helpers") + `()`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestLiveStructuralUnionReusesSecondTypedMember(t *testing.T) {
	p := typedStructuralUnionProgram()
	n := hir.NewNames()
	main := `typed:=&` + n.Get("struct.probe.ts.JsDeclarations") + `{` + n.Get("member.js") + `:true};record:=parseJSON(str("{}"));record.put(str("helpers"),box(typed));view:=` + entry("probe.ts", "view") + `(record);if view.` + n.Get("view.read.probe.ts.Container.helpers") + `()!=typed {panic("second typed union member identity changed")};fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
