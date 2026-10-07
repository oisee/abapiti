package hir

import (
	"strings"
	"testing"
)

func TestVerifier(t *testing.T) {
	i := T(I32)
	good := func() *Program {
		return &Program{Classes: []*Class{{Name: "C", Methods: []*Method{{Name: "f", Params: []Param{{"a", i}}, Result: i, Body: B(&Stmt{Kind: Return, X: V("a", i)})}}}}}
	}
	cases := []struct {
		name   string
		mutate func(*Program)
		want   string
	}{
		{"unresolved", func(p *Program) { p.Classes[0].Methods[0].Body.List[0].X.Name = "missing" }, "unresolved local"},
		{"assignment", func(p *Program) {
			p.Classes[0].Methods[0].Body.List = []*Stmt{{Node: Node{ID: 42}, Kind: Assign, X: V("a", i), Y: L(T(String), "bad")}, {Kind: Return, X: V("a", i)}}
		}, "node 42"},
		{"call", func(p *Program) {
			p.Classes[0].Methods[0].Body.List[0].X = &Expr{Node: Node{ID: 9}, Kind: DirectCall, X: &Expr{Kind: This, Type: Ref("C")}, Name: "f", Type: i, Args: []*Expr{L(T(String), "bad")}}
		}, "call argument type mismatch"},
		{"virtual", func(p *Program) {
			p.Classes[0].Methods[0].Body.List[0].X = &Expr{Kind: VirtualCall, X: &Expr{Kind: This, Type: Ref("C")}, Name: "f", Type: i, Args: []*Expr{L(i, 1)}}
		}, "virtual call on non-virtual"},
		{"virtual receiver", func(p *Program) {
			p.Classes[0].Methods[0].Virtual = true
			p.Classes[0].Methods[0].Body.List[0].X = &Expr{Node: Node{ID: 23, Source: "call.ts:8"}, Kind: VirtualCall, Owner: "C", Name: "f", Type: i, Args: []*Expr{L(i, 1)}}
		}, "node 23 (call.ts:8): virtual call requires object receiver"},
		{"virtual non-object receiver", func(p *Program) {
			p.Classes[0].Methods[0].Body.List[0].X = &Expr{Kind: VirtualCall, X: L(i, 1), Name: "f", Type: i}
		}, "call on non-object"},
		{"void equality", func(p *Program) {
			p.Classes[0].Methods = append(p.Classes[0].Methods, &Method{Name: "noop", Static: true, Result: T(Void), Body: B()})
			call := func() *Expr { return &Expr{Kind: DirectCall, Owner: "C", Name: "noop", Type: T(Void)} }
			p.Classes[0].Methods[0].Body.List = append([]*Stmt{{Kind: ExprStmt, X: &Expr{Node: Node{ID: 24, Source: "eq.ts:9"}, Kind: Binary, Op: "==", Type: T(Bool), X: call(), Y: call()}}}, p.Classes[0].Methods[0].Body.List...)
		}, "node 24 (eq.ts:9): void equality operand"},
		{"void inequality", func(p *Program) {
			p.Classes[0].Methods = append(p.Classes[0].Methods, &Method{Name: "noop", Static: true, Result: T(Void), Body: B()})
			call := func() *Expr { return &Expr{Kind: DirectCall, Owner: "C", Name: "noop", Type: T(Void)} }
			p.Classes[0].Methods[0].Body.List = append([]*Stmt{{Kind: ExprStmt, X: &Expr{Kind: Binary, Op: "!=", Type: T(Bool), X: call(), Y: call()}}}, p.Classes[0].Methods[0].Body.List...)
		}, "void equality operand"},
		{"nil class method", func(p *Program) {
			p.Classes[0].Node = Node{ID: 25, Source: "class.ts:1"}
			p.Classes[0].Methods = append(p.Classes[0].Methods, nil)
		}, "node 25 (class.ts:1): nil class method"},
		{"nil interface method", func(p *Program) {
			p.Interfaces = []*Interface{{Node: Node{ID: 26, Source: "interface.ts:1"}, Name: "I", Methods: []*Method{nil}}}
		}, "node 26 (interface.ts:1): nil interface method"},
		{"abstract", func(p *Program) {
			p.Classes[0].Abstract = true
			p.Classes[0].Methods[0].Abstract = true
			p.Classes[0].Methods[0].Virtual = true
			p.Classes[0].Methods[0].Body = nil
			p.Classes = append(p.Classes, &Class{Name: "D", Super: "C"})
		}, "missing abstract implementation"},
		{"cycle", func(p *Program) { p.Classes[0].Super = "C" }, "inheritance cycle"},
		{"condition", func(p *Program) {
			p.Classes[0].Methods[0].Body.List = append([]*Stmt{{Kind: If, X: L(i, 1), Body: B()}}, p.Classes[0].Methods[0].Body.List...)
		}, "condition must be bool"},
		{"runtime", func(p *Program) {
			p.Classes[0].Methods[0].Body.List[0].X = &Expr{Kind: RuntimeOp, Type: i, Op: "regex.match", X: L(T(String), "x")}
		}, "unsupported runtime op"},
		{"break", func(p *Program) {
			p.Classes[0].Methods[0].Body.List = append([]*Stmt{{Kind: Break}}, p.Classes[0].Methods[0].Body.List...)
		}, "loop control outside loop"},
	}
	if es := Verify(good()); len(es) != 0 {
		t.Fatal(es)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := good()
			tc.mutate(p)
			es := Verify(p)
			found := false
			for _, e := range es {
				found = found || strings.Contains(e.Error(), tc.want)
			}
			if !found {
				t.Fatalf("want %s, got %v", tc.want, es)
			}
		})
	}
}
func TestNames(t *testing.T) {
	ids := []string{"class.with.a.very.long.name", "CLASS.WITH.A.VERY.LONG.NAME", "member.a-b", "member.a_b", "こんにちは", "constructor"}
	a, b := NewNames(), NewNames()
	seen := map[string]bool{}
	for i := len(ids) - 1; i >= 0; i-- {
		b.Get(ids[i])
	}
	for _, id := range ids {
		n := a.Get(id)
		if len(n) > 30 || seen[n] || n != b.Get(id) {
			t.Fatalf("invalid name %s", n)
		}
		seen[n] = true
	}
}
func TestRuntimeCatalogue(t *testing.T) {
	for op, s := range RuntimeSpecs {
		typ := T(s.Receiver)
		switch s.Receiver {
		case Array, OrderedSet:
			typ.Args = []Type{T(I32)}
		case OrderedMap:
			typ.Args = []Type{T(I32), T(String)}
		}
		ps, r, ok := RuntimeSignature(op, typ)
		if !ok || len(ps) != s.Arity || r.Kind == "" {
			t.Fatal(op)
		}
	}
	if _, _, ok := RuntimeSignature("array.get", T(Array)); ok {
		t.Fatal("malformed generic accepted")
	}
}
