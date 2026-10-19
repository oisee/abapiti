package hir

import "testing"

func TestStructuralUnionMembershipMetadata(t *testing.T) {
	row := func(name string) *Class {
		return &Class{Name: name, Structural: true, Implements: []string{"Rows"}, Fields: []Field{{Name: "kind", Type: T(String)}}}
	}
	a, b := row("A"), row("B")
	member := func(name, tag string) StructuralMember {
		return StructuralMember{Type: Ref(name), Fields: []StructuralField{{Name: "kind", Type: T(String), Domains: []StructuralDomain{{Kind: String, Exact: true, Literal: tag}}}}}
	}
	iface := &Interface{Name: "Rows", StructuralMembers: []StructuralMember{member("A", "a"), member("B", "b")}}
	p := &Program{Classes: []*Class{a, b}, Interfaces: []*Interface{iface}}
	if errors := Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	for _, change := range []func(){
		func() { a.Structural = false },
		func() { a.Implements = nil },
		func() { iface.StructuralMembers[0].Fields = nil },
		func() { iface.StructuralMembers[0].Fields[0].Type = T(Number) },
		func() { iface.StructuralMembers[0].Fields[0].Type = T(Optional) },
		func() {
			iface.StructuralMembers[0].Fields[0].Domains = []StructuralDomain{{Kind: Number, Exact: true, Literal: "1); panic(0)"}}
		},
	} {
		a.Structural = true
		a.Implements = []string{"Rows"}
		iface.StructuralMembers[0] = member("A", "a")
		change()
		if len(Verify(p)) == 0 {
			t.Fatal("invalid member metadata accepted")
		}
	}
}

func TestStructuralUnionMemberProjectionTyping(t *testing.T) {
	a := &Class{Name: "A", Structural: true, Implements: []string{"Rows"}}
	b := &Class{Name: "B", Structural: true, Implements: []string{"Rows"}}
	projection := &Expr{Kind: RuntimeOp, Op: "dynamic.asRef", Owner: "Rows", Type: Ref("B"), X: V("d", T(Dynamic))}
	method := &Method{Name: "read", Static: true, Params: []Param{{Name: "d", Type: T(Dynamic)}}, Result: Ref("B"), Body: B(&Stmt{Kind: Return, X: projection})}
	p := &Program{Classes: []*Class{a, b, {Name: "Use", Methods: []*Method{method}}}, Interfaces: []*Interface{{Name: "Rows", StructuralMembers: []StructuralMember{{Type: Ref("A")}, {Type: Ref("B")}}}}}
	if errors := Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	projection.Owner = "missing"
	if len(Verify(p)) == 0 {
		t.Fatal("unknown family projection accepted")
	}
	projection.Owner = "Rows"
	projection.Type = Ref("Use")
	method.Result = projection.Type
	if len(Verify(p)) == 0 {
		t.Fatal("unrelated member projection accepted")
	}
}

// Optional storage carries undefined, never null. Null needs tagged Dynamic storage.
func TestStructuralUnionNullDomainStorage(t *testing.T) {
	for _, tc := range []struct {
		name     string
		typ      Type
		accepted bool
	}{
		{"string", T(String), false}, {"optional-string", T(Optional, T(String)), false}, {"dynamic", T(Dynamic), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			member := func(name string) StructuralMember {
				return StructuralMember{Type: Ref(name), Fields: []StructuralField{{Name: "value", Type: tc.typ, Domains: []StructuralDomain{{Kind: "null"}}}}}
			}
			row := func(name string) *Class {
				return &Class{Name: name, Structural: true, Implements: []string{"Rows"}, Fields: []Field{{Name: "value", Type: tc.typ}}}
			}
			p := &Program{Classes: []*Class{row("A"), row("B")}, Interfaces: []*Interface{{Name: "Rows", StructuralMembers: []StructuralMember{member("A"), member("B")}}}}
			errors := Verify(p)
			if (len(errors) == 0) != tc.accepted {
				t.Fatalf("accepted=%v errors=%v", tc.accepted, errors)
			}
		})
	}
}
