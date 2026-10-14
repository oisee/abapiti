package hir

import (
	"encoding/json"
	"strings"
	"testing"
)

func unionContractProgram() *Program {
	str := T(String)
	ref := Ref("Leaf")
	u := &UnionDecl{Name: "Tree", Discriminant: "k", Complete: true, Alternatives: []UnionAlternative{{ID: "leaf", Tags: []string{"leaf"}, Payload: ref}}}
	c := &Class{Name: ref.Name, Structural: true, Fields: []Field{{Name: "k", Type: str}, {Name: "value", Type: T(Number)}}}
	c.Ctor = &Method{Name: "constructor", Params: []Param{{Name: "k", Type: str}, {Name: "value", Type: T(Number)}}, Result: T(Void), Body: B()}
	newLeaf := &Expr{Kind: New, Type: ref, Args: []*Expr{L(str, "leaf"), L(T(Number), 1)}}
	admit := &Expr{Kind: RuntimeOp, Op: "union.admit", Owner: u.Name, Name: "leaf", Type: Type{Kind: UnionRef, Name: u.Name}, X: newLeaf}
	c.Methods = []*Method{{Name: "make", Static: true, Result: admit.Type, Body: B(&Stmt{Kind: Return, X: admit})}}
	return &Program{Classes: []*Class{c}, Unions: []*UnionDecl{u}}
}

func TestUnionContractRoundTrip(t *testing.T) {
	p := unionContractProgram()
	if errs := Verify(p); len(errs) != 0 {
		t.Fatal(errs)
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var restored Program
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if errs := Verify(&restored); len(errs) != 0 {
		t.Fatal(errs)
	}
	if Dump(p) != Dump(&restored) {
		t.Fatal("family membership changed in serialization")
	}
	if UsedRuntimeOps(&restored)["union.declarations"].Source != p.Unions[0].Source {
		t.Fatal("lost declaration boundary")
	}
}

func TestUnionContractRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*Program)
	}{
		{"incomplete", "incomplete union", func(p *Program) { p.Unions[0].Complete = false }},
		{"missing-layout", "unresolved class", func(p *Program) { p.Unions[0].Alternatives[0].Payload = Ref("Missing") }},
		{"optional-tag", "required string", func(p *Program) { p.Classes[0].Fields[0].Type = T(Optional, T(String)) }},
		{"wrong-tag", "wrong literal tag", func(p *Program) { p.Classes[0].Methods[0].Body.List[0].X.X.Args[0].Value = "branch" }},
		{"missing-required-field", "call arity mismatch", func(p *Program) {
			p.Classes[0].Methods[0].Body.List[0].X.X.Args = p.Classes[0].Methods[0].Body.List[0].X.X.Args[:1]
		}},
		{"wrong-payload", "payload mismatch", func(p *Program) { p.Classes[0].Methods[0].Body.List[0].X.Type.Name = "Other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := unionContractProgram()
			tc.change(p)
			for _, err := range Verify(p) {
				if strings.Contains(err.Error(), tc.want) {
					return
				}
			}
			t.Fatalf("missing refusal %q: %v", tc.want, Verify(p))
		})
	}
}
