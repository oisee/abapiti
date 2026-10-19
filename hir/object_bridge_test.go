package hir

import "testing"

func TestObjectBridgeSignature(t *testing.T) {
	op := &Expr{Kind: RuntimeOp, Op: "dynamic.asObject", Type: Ref(RootObject), X: V("d", T(Dynamic))}
	m := &Method{Name: "bridge", Static: true, Params: []Param{{Name: "d", Type: T(Dynamic)}}, Result: op.Type, Body: B(&Stmt{Kind: Return, X: op})}
	p := &Program{Classes: []*Class{{Name: "Nominal"}, {Name: "Probe", Methods: []*Method{m}}}}
	if errs := Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, change := range []func(){func() { op.Type = Ref("Nominal"); m.Result = op.Type }, func() { op.X = V("d", Ref(RootObject)); m.Params[0].Type = op.X.Type }, func() { op.Owner = "Nominal" }, func() { op.Args = []*Expr{L(T(I32), 0)} }} {
		op.Type = Ref(RootObject)
		m.Result = op.Type
		op.X = V("d", T(Dynamic))
		m.Params[0].Type = T(Dynamic)
		op.Owner = ""
		op.Args = nil
		change()
		if len(Verify(p)) == 0 {
			t.Fatal("invalid object bridge signature accepted")
		}
	}
}
