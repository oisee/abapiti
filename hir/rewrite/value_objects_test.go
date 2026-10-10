package rewrite_test

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
	"testing"
)

func valueFixture() *hir.Program {
	n := hir.T(hir.I32)
	c := &hir.Class{Node: hir.Node{Source: "fixture.ts:1:1"}, Name: "C", Fields: []hir.Field{{Node: hir.Node{Source: "fixture.ts:2:1"}, Name: "n", Type: n}}}
	c.Ctor = &hir.Method{Node: hir.Node{Source: "fixture.ts:3:1"}, Name: "constructor", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Node: hir.Node{Source: "fixture.ts:4:1"}, Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Owner: "C", Name: "n", Type: n, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}, Y: hir.L(n, 1)})}
	return &hir.Program{Classes: []*hir.Class{c}}
}
func TestValueObjects(t *testing.T) {
	cases := []struct {
		name, condition string
		change          func(*hir.Program)
	}{
		{"positive", "", func(p *hir.Program) {}},
		{"mutated-after-ctor", "1", func(p *hir.Program) {
			p.Classes[0].Methods = []*hir.Method{{Name: "mutate", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Owner: "C", Name: "n", Type: hir.T(hir.I32), X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}, Y: hir.L(hir.T(hir.I32), 2)})}}
		}},
		{"alias-mutation", "1", func(p *hir.Program) {
			p.Classes[0].Methods = []*hir.Method{{Name: "mutate", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "alias", Type: hir.Ref("C"), X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}, &hir.Stmt{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Owner: "C", Name: "n", Type: hir.T(hir.I32), X: hir.V("alias", hir.Ref("C"))}, Y: hir.L(hir.T(hir.I32), 2)})}}
		}},
		{"identity-compare", "2", func(p *hir.Program) {
			p.Classes[0].Methods = []*hir.Method{{Name: "compare", Static: true, Params: []hir.Param{{Name: "a", Type: hir.Ref("C")}, {Name: "b", Type: hir.Ref("C")}}, Result: hir.T(hir.Bool), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: hir.V("a", hir.Ref("C")), Y: hir.V("b", hir.Ref("C"))}})}}
		}},
		{"instanceof", "2", func(p *hir.Program) {
			p.Classes[0].Methods = []*hir.Method{{Name: "test", Static: true, Params: []hir.Param{{Name: "a", Type: hir.Ref("C")}}, Result: hir.T(hir.Bool), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.InstanceOf, Owner: "C", Type: hir.T(hir.Bool), X: hir.V("a", hir.Ref("C"))}})}}
		}},
		{"subclass", "3", func(p *hir.Program) {
			p.Classes = append(p.Classes, &hir.Class{Node: hir.Node{Source: "fixture.ts:8:1"}, Name: "D", Super: "C"})
		}},
		{"map-key", "2", func(p *hir.Program) {
			mt := hir.T(hir.OrderedMap, hir.Ref("C"), hir.T(hir.I32))
			p.Classes[0].Methods = []*hir.Method{{Name: "key", Static: true, Params: []hir.Param{{Name: "a", Type: hir.Ref("C")}, {Name: "map", Type: mt}}, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.RuntimeOp, Op: "map.set", Type: mt, X: hir.V("map", mt), Args: []*hir.Expr{hir.V("a", hir.Ref("C")), hir.L(hir.T(hir.I32), 1)}}})}}
		}},
		{"leaked-this", "1", func(p *hir.Program) {
			c := p.Classes[0]
			c.Fields = append(c.Fields, hir.Field{Name: "published", Type: hir.Ref("C"), Static: true})
			c.Ctor.Body.List = append([]*hir.Stmt{{Node: hir.Node{Source: "fixture.ts:3:5"}, Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "C", Name: "published", Type: hir.Ref("C")}, Y: &hir.Expr{Node: hir.Node{Source: "fixture.ts:3:10"}, Kind: hir.This, Type: hir.Ref("C")}}}, c.Ctor.Body.List...)
		}},
		{"optional-positive", "", func(p *hir.Program) {
			p.Classes[0].Methods = []*hir.Method{{Name: "optional", Static: true, Params: []hir.Param{{Name: "a", Type: hir.T(hir.Optional, hir.Ref("C"))}}, Result: hir.T(hir.Void), Body: hir.B()}}
		}},
	}
	cases = append(cases, struct {
		name, condition string
		change          func(*hir.Program)
	}{"dynamic-equality", "2", func(p *hir.Program) {
		p.Classes[0].Methods = []*hir.Method{{Name: "compare", Static: true, Params: []hir.Param{{Name: "a", Type: hir.T(hir.Dynamic)}, {Name: "b", Type: hir.T(hir.Dynamic)}}, Result: hir.T(hir.Bool), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.RuntimeOp, Op: "dynamic.strictEquals", Type: hir.T(hir.Bool), X: hir.V("a", hir.T(hir.Dynamic)), Args: []*hir.Expr{hir.V("b", hir.T(hir.Dynamic))}}})}}
	}})
	cases = append(cases, struct {
		name, condition string
		change          func(*hir.Program)
	}{"primitive-equality", "", func(p *hir.Program) {
		p.Classes[0].Methods = []*hir.Method{{Name: "equals", Result: hir.T(hir.Bool), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: hir.L(hir.T(hir.I32), 1), Y: hir.L(hir.T(hir.I32), 1)}})}}
	}})

	cases = append(cases, struct {
		name, condition string
		change          func(*hir.Program)
	}{"array-reference-membership", "2", func(p *hir.Program) {
		at := hir.T(hir.Array, hir.Ref("C"))
		p.Classes[0].Methods = []*hir.Method{{Name: "contains", Static: true, Params: []hir.Param{{Name: "a", Type: at}, {Name: "item", Type: hir.Ref("C")}}, Result: hir.T(hir.Bool), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.RuntimeOp, Op: "array.includes", Type: hir.T(hir.Bool), X: hir.V("a", at), Args: []*hir.Expr{hir.V("item", hir.Ref("C"))}}})}}
	}})

	cases = append(cases, struct {
		name, condition string
		change          func(*hir.Program)
	}{"optional-container-positive", "", func(p *hir.Program) {
		p.Classes[0].Methods = []*hir.Method{{Name: "container", Static: true, Params: []hir.Param{{Name: "a", Type: hir.T(hir.Optional, hir.T(hir.Array, hir.Ref("C")))}}, Result: hir.T(hir.Void), Body: hir.B()}}
	}})

	cases = append(cases, struct {
		name, condition string
		change          func(*hir.Program)
	}{"static-field-mutation", "1", func(p *hir.Program) {
		c := p.Classes[0]
		c.Fields = append(c.Fields, hir.Field{Name: "counter", Type: hir.T(hir.I32), Static: true})
		c.Methods = []*hir.Method{{Name: "count", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Node: hir.Node{Source: "fixture.ts:8:1"}, Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "C", Name: "counter", Type: hir.T(hir.I32)}, Y: hir.L(hir.T(hir.I32), 2)})}}
	}})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valueFixture()
			tc.change(p)
			before := hir.Dump(p)
			flow, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			base := rewrite.ExtractValueObjects(p, flow)
			db := gracecheck.Evaluate(t, base, rewrite.ValueObjectRules())
			if tc.condition == "" {
				if !db.Has("value_object", "C") {
					t.Fatalf("positive rejected: %v", db.Facts("vo_violation"))
				}
			} else {
				if !db.Has("vo_blocked", "C", tc.condition) || db.Has("value_object", "C") {
					t.Fatalf("negative qualified: %v", db.Facts("vo_violation"))
				}
				found := false
				for _, f := range db.Facts("vo_violation") {
					if f[0] == "C" && f[1] == tc.condition && f[3] != "" {
						found = true
					}
				}
				if !found {
					t.Fatal("missing source provenance")
				}
			}
			if tc.name == "optional-container-positive" && db.Has("vo_absent_flag", "C") {
				t.Fatal("container absence incorrectly applied to its rows")
			}
			if tc.name == "optional-positive" && !db.Has("vo_absent_flag", "C") {
				t.Fatal("missing absent flag")
			}
			if hir.Dump(p) != before {
				t.Fatal("screen mutated HIR")
			}
		})
	}
}
