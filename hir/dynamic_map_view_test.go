package hir

import "testing"

func TestDynamicMapViewCheckedBoundary(t *testing.T) {
	dyn := T(Dynamic)
	target := T(OrderedMap, T(String), T(Number))
	view := &Expr{Kind: RuntimeOp, Op: "dynamic.mapView", Type: target, X: V("d", dyn)}
	get := &Expr{Kind: RuntimeOp, Op: "map.get", Type: T(Optional, T(Number)), X: V("m", target), Args: []*Expr{L(T(String), "x")}}
	m := &Method{Name: "run", Static: true, Params: []Param{{Name: "d", Type: dyn}}, Result: T(Void), Body: B(&Stmt{Kind: VarDecl, Name: "m", Type: target, X: view}, &Stmt{Kind: ExprStmt, X: get})}
	p := &Program{Classes: []*Class{{Name: "Probe", Methods: []*Method{m}}}}
	if len(Verify(p)) == 0 || len(VerifyBody(p, p.Classes[0], m)) == 0 {
		t.Fatal("unchecked access to erased Map accepted")
	}
	SealCheckedMapAccess(p)
	if get.Op != "map.checkedGet" {
		t.Fatal(get.Op)
	}
	if errs := Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, change := range []func(){func() { view.Type = T(OrderedMap, T(Number), T(Number)) }, func() { view.X = V("d", Ref(RootObject)); m.Params[0].Type = view.X.Type }, func() { view.Type.Name = "unqualified" }, func() { view.Owner = "Probe" }, func() { view.Args = []*Expr{L(T(I32), 0)} }} {
		view.Type = target
		view.X = V("d", dyn)
		m.Params[0].Type = dyn
		view.Owner = ""
		view.Args = nil
		change()
		if len(Verify(p)) == 0 {
			t.Fatal("invalid Map bridge signature accepted")
		}
	}
}
