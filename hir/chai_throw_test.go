package hir

import "testing"

func TestChaiThrowCallableBoundary(t *testing.T) {
	thunk := Type{Kind: InterfaceRef, Name: "closure.thunk"}
	for _, receiver := range []Type{thunk, T(Dynamic), T(String), T(Number)} {
		for _, argument := range []Type{T(String), T(Number)} {
			op := &Expr{Kind: RuntimeOp, Op: "chai.throwMatch", X: V("action", receiver), Args: []*Expr{V("expected", argument)}, Type: T(Bool)}
			p := &Program{Interfaces: []*Interface{{Name: thunk.Name, Methods: []*Method{{Name: "call", Result: T(Void)}}}}, Classes: []*Class{{Name: "Probe", Methods: []*Method{{Name: "run", Static: true, Params: []Param{{Name: "action", Type: receiver}, {Name: "expected", Type: argument}}, Result: T(Bool), Body: B(&Stmt{Kind: Return, X: op})}}}}}
			want := (receiver.Equal(thunk) || receiver.Kind == Dynamic) && argument.Kind == String
			if errors := Verify(p); (len(errors) == 0) != want {
				t.Fatalf("receiver=%s argument=%s: %v", receiver, argument, errors)
			}
		}
	}
}
