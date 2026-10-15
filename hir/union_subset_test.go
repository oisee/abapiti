package hir

import (
	"strings"
	"testing"
)

func TestUnionSubsetContracts(t *testing.T) {
	p := unionContractProgram()
	full := p.Unions[0]
	full.Alternatives[0].Tags = []string{"a", "b"}
	full.Alternatives[0].Literals = []UnionLiteral{{Field: "k", Values: []string{"a", "b"}}}
	subset := *full
	subset.Name = "OnlyA"
	subset.Alternatives = append([]UnionAlternative(nil), full.Alternatives...)
	subset.Alternatives[0].Tags = []string{"a"}
	subset.Alternatives[0].Literals = []UnionLiteral{{Field: "k", Values: []string{"a"}}}
	p.Unions = append(p.Unions, &subset)
	if !UnionIncludes(full, &subset) || UnionIncludes(&subset, full) {
		t.Fatal("literal-domain subset inclusion is not directional")
	}
	e := p.Classes[0].Methods[0].Body.List[0].X
	e.X.Args[0].Value = "a"
	source := &Expr{Kind: RuntimeOp, Op: "union.project", Type: Type{Kind: UnionRef, Name: subset.Name}, X: e}
	widen := &Expr{Kind: RuntimeOp, Op: "union.widen", Type: e.Type, X: source}
	p.Classes[0].Methods[0].Body.List[0].X = widen
	if errors := Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	source.Type.Name = "Missing"
	if errors := Verify(p); len(errors) == 0 {
		t.Fatal("unresolved subset projection accepted")
	}
}

func TestUnionLiteralContractRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		literal UnionLiteral
	}{
		{"missing-field", UnionLiteral{Field: "missing", Values: []string{"a"}}},
		{"wrong-storage", UnionLiteral{Field: "value", Values: []string{"a"}}},
		{"empty-domain", UnionLiteral{Field: "k"}},
		{"duplicate-domain", UnionLiteral{Field: "k", Values: []string{"leaf", "leaf"}}},
		{"tag-mismatch", UnionLiteral{Field: "k", Values: []string{"other"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := unionContractProgram()
			p.Unions[0].Alternatives[0].Literals = []UnionLiteral{tc.literal}
			for _, err := range Verify(p) {
				if strings.Contains(err.Error(), "union literal") || strings.Contains(err.Error(), "literal domain") {
					return
				}
			}
			t.Fatal("malformed domain accepted", Verify(p))
		})
	}
}
