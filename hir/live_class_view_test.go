package hir

import "testing"

func TestDataClassViewBoundary(t *testing.T) {
	base := &Class{Name: "Base", Abstract: true, Fields: []Field{{Name: "optional", Type: T(Optional, T(String))}}}
	child := &Class{Name: "Child", Super: "Base", Fields: []Field{{Name: "value", Type: T(Number)}}}
	p := &Program{Classes: []*Class{base, child}}
	if !DataClassView(p, "Child") {
		t.Fatal("public data ancestry must support a live view")
	}
	base.Fields[0].Private = true
	if DataClassView(p, "Child") {
		t.Fatal("inherited private fields must refuse")
	}
	base.Fields[0].Private = false
	base.ViewUnsafe = true
	if DataClassView(p, "Child") {
		t.Fatal("accessors must refuse even when their lowering was skipped")
	}
	base.ViewUnsafe = false
	base.Methods = []*Method{{Name: "read", Result: T(Number), Body: B(&Stmt{Kind: Return, X: &Expr{Kind: This, Type: Ref("Base")}})}}
	if DataClassView(p, "Child") {
		t.Fatal("prototype method dispatch must refuse")
	}
	base.Methods = []*Method{{Name: "read", Result: T(Optional, T(String)), Body: B(&Stmt{Kind: Return, X: &Expr{Kind: FieldGet, Name: "optional", Type: T(Optional, T(String)), X: &Expr{Kind: This, Type: Ref("Base")}}})}}
	if !DataClassView(p, "Child") {
		t.Fatal("unused field reader is safe")
	}
	p.Classes = append(p.Classes, &Class{Name: "Consumer", Methods: []*Method{{Name: "use", Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: VirtualCall, Name: "read", Type: T(Optional, T(String)), X: V("conf", Ref("Child"))}})}}})
	if DataClassView(p, "Child") {
		t.Fatal("prototype dispatch must refuse before inlining")
	}
	child.Methods = append(child.Methods, &Method{Name: "set", Static: true, Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "dynamic.classView", Type: Ref("Child"), X: V("input", T(Dynamic))}})})
	SealLiveClassViewBoundary(p)
	p.Classes = p.Classes[:2] // Simulate elimination of the dispatch by inlining.
	if DataClassView(p, "Child") {
		t.Fatal("inlining must not erase the prototype refusal")
	}
	child.ViewUnsafe = false
	base.Methods = nil
	child.Super = "Missing"
	if DataClassView(p, "Child") {
		t.Fatal("unknown ancestry must refuse")
	}
}
