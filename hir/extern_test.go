package hir

import "testing"

func TestExternVerification(t *testing.T) {
	makeCall := func() *Expr {
		return &Expr{Node: Node{Source: "probe.ts:1:1"}, Kind: ExternCall, Owner: "example.org/host", Name: "Read", Type: T(String), Extern: &ExternSignature{Params: []Type{T(String)}}, Args: []*Expr{L(T(String), "file")}}
	}
	for _, c := range []struct {
		name   string
		change func(*Expr)
		valid  bool
	}{
		{"valid", func(*Expr) {}, true},
		{"missing signature", func(e *Expr) { e.Extern = nil }, false},
		{"missing name", func(e *Expr) { e.Name = "" }, false},
		{"wrong arity", func(e *Expr) { e.Args = nil }, false},
		{"wrong argument", func(e *Expr) { e.Args[0] = L(T(Bool), true) }, false},
		{"missing argument", func(e *Expr) { e.Args[0] = nil }, false},
		{"unsupported result", func(e *Expr) { e.Type = T(Dynamic) }, false},
		{"invalid rest", func(e *Expr) { e.Extern.Rest = true }, false},
		{"empty rest signature", func(e *Expr) { e.Extern.Rest = true; e.Extern.Params = nil }, false},
		{"invalid spread", func(e *Expr) { e.Extern.Spread = true }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := makeCall()
			c.change(e)
			m := &Method{Name: "run", Static: true, Result: e.Type, Body: B(&Stmt{Kind: Return, X: e})}
			p := &Program{Classes: []*Class{{Name: "Probe", Methods: []*Method{m}}}}
			errs := Verify(p)
			if (len(errs) == 0) != c.valid {
				t.Fatalf("valid=%v: %v", c.valid, errs)
			}
		})
	}
}
