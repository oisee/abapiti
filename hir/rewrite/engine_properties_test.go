package rewrite_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
)

// Each graph has a forced cycle of length 1..4 plus seeded cross edges. Virtual
// methods exercise both analysis dispatch and the inliner's cycle exclusion.
func graph(seed int64, n, cycle int) *hir.Program {
	r := rand.New(rand.NewSource(seed))
	c := &hir.Class{Name: "Graph"}
	i := hir.T(hir.I32)
	vc := func(target int) *hir.Expr {
		return &hir.Expr{Kind: hir.VirtualCall, Name: fmt.Sprintf("m%d", target), Type: i, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Graph")}}
	}
	for j := 0; j < n; j++ {
		var ss []*hir.Stmt
		if j < cycle {
			ss = append(ss, &hir.Stmt{Kind: hir.ExprStmt, X: vc((j + 1) % cycle)})
		}
		for k := 0; k < r.Intn(3); k++ {
			ss = append(ss, &hir.Stmt{Kind: hir.ExprStmt, X: vc(r.Intn(n))})
		}
		ss = append(ss, &hir.Stmt{Kind: hir.Return, X: hir.L(i, j)})
		c.Methods = append(c.Methods, &hir.Method{Name: fmt.Sprintf("m%d", j), Virtual: true, Result: i, Body: hir.B(ss...)})
	}
	return &hir.Program{Classes: []*hir.Class{c, {Name: "Empty"}}}
}
func TestSeededEnginePrograms(t *testing.T) {
	for seed := int64(0); seed < 32; seed++ {
		t.Run(fmt.Sprintf("seed_%02d", seed), func(t *testing.T) {
			p := graph(seed, 8, 1+int(seed%4))
			gracecheck.Check(t, p)
		})
	}
}
func TestSmallAndNestedEnginePrograms(t *testing.T) {
	cases := map[string]*hir.Program{"empty": {}, "single": {Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "one", Static: true, Result: hir.T(hir.Void), Body: hir.B()}}}}}}
	for _, depth := range []int{0, 1, 3, 4, 5, 63, 64, 65} {
		s := &hir.Stmt{Kind: hir.Throw, X: hir.L(hir.T(hir.I32), 1)}
		for i := 0; i < depth; i++ {
			s = &hir.Stmt{Kind: hir.While, X: hir.L(hir.T(hir.Bool), false), Body: hir.B(s)}
		}
		cases[fmt.Sprintf("nest_%d", depth)] = &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "raise", Static: true, Result: hir.T(hir.Void), Body: hir.B(s)}, {Name: "caller", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Owner: "C", Name: "raise", Type: hir.T(hir.Void)}})}}}}}
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) { gracecheck.Check(t, p) })
	}
}
func TestRecursiveInliningBound(t *testing.T) {
	for cycle := 1; cycle <= 4; cycle++ {
		for _, depth := range []int{0, 1, 2, 4, 64} {
			t.Run(fmt.Sprintf("cycle%d_depth%d", cycle, depth), func(t *testing.T) {
				p := graph(42, cycle, cycle)
				before := hir.Dump(p)
				src := strings.ReplaceAll(gracecheck.Source(t, "inline"), "depth 64", fmt.Sprintf("depth %d", depth))
				_, rs, e := rewrite.Parse(src)
				if e != nil {
					t.Fatal(e)
				}
				s, e := rewrite.Rewrite(p, rs, rewrite.Limits{Rounds: 8})
				if e != nil {
					t.Fatal(e)
				}
				if s.CallSites != 0 || s.Rounds != 1 || hir.Dump(p) != before {
					t.Fatalf("recursive cycle expanded: %+v", s)
				}
			})
		}
	}
}

func TestDiamondInterfacesAndCollectionAliases(t *testing.T) {
	i := hir.T(hir.I32)
	v := hir.T(hir.Void)
	f := func() *hir.Method { return &hir.Method{Name: "f", Virtual: true, Result: v, Body: hir.B()} }
	base := &hir.Class{Name: "Base", Methods: []*hir.Method{f()}}
	left := &hir.Class{Name: "Left", Super: "Base", Implements: []string{"LeftI"}}
	right := &hir.Class{Name: "Right", Super: "Base", Implements: []string{"RightI"}, Methods: []*hir.Method{f()}}
	leaf := &hir.Class{Name: "Leaf", Super: "Left", Implements: []string{"LeftI", "RightI", "DiamondI"}}
	// HIR has single class inheritance and structural interface compatibility;
	// this represents the shared-base diamond through its interface arms.
	p := &hir.Program{Classes: []*hir.Class{base, left, right, leaf}, Interfaces: []*hir.Interface{{Name: "LeftI", Methods: []*hir.Method{f()}}, {Name: "RightI", Methods: []*hir.Method{f()}}, {Name: "DiamondI", Methods: []*hir.Method{f()}}}}
	caller := &hir.Method{Name: "invoke", Static: true, Params: []hir.Param{{Name: "x", Type: hir.Type{Kind: hir.InterfaceRef, Name: "DiamondI"}}}, Result: v, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.VirtualCall, Name: "f", Type: v, X: hir.V("x", hir.Type{Kind: hir.InterfaceRef, Name: "DiamondI"})}})}
	p.Classes = append(p.Classes, &hir.Class{Name: "Caller", Methods: []*hir.Method{caller}})
	t.Run("diamond", func(t *testing.T) { gracecheck.Check(t, p) })
	for _, collection := range []hir.Type{hir.T(hir.Array, i), hir.T(hir.OrderedMap, i, i)} {
		for _, optional := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_optional_%v", collection, optional), func(t *testing.T) {
				ty := collection
				if optional {
					ty = hir.T(hir.Optional, ty)
				}
				m := &hir.Method{Name: "aliases", Static: true, Params: []hir.Param{{Name: "p", Type: ty}}, Result: ty, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: ty, X: hir.V("p", ty)}, &hir.Stmt{Kind: hir.VarDecl, Name: "b", Type: ty, X: hir.V("a", ty)}, &hir.Stmt{Kind: hir.Return, X: hir.V("b", ty)})}
				gracecheck.Check(t, &hir.Program{Classes: []*hir.Class{{Name: "Alias", Methods: []*hir.Method{m}}}})
			})
		}
	}
}

func TestIndependentStatementPermutations(t *testing.T) {
	for seed := int64(0); seed < 32; seed++ {
		t.Run(fmt.Sprintf("seed_%02d", seed), func(t *testing.T) {
			i := hir.T(hir.I32)
			m := &hir.Method{Name: "literals", Static: true, Result: hir.T(hir.Void), Body: hir.B()}
			for n := 0; n < 8; n++ {
				m.Body.List = append(m.Body.List, &hir.Stmt{Kind: hir.ExprStmt, X: hir.L(i, n)})
			}
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
			q := gracecheck.Clone(t, p)
			r := rand.New(rand.NewSource(seed))
			r.Shuffle(8, func(i, j int) {
				q.Classes[0].Methods[0].Body.List[i], q.Classes[0].Methods[0].Body.List[j] = q.Classes[0].Methods[0].Body.List[j], q.Classes[0].Methods[0].Body.List[i]
			})
			a, e := rewrite.Analyze(p)
			if e != nil {
				t.Fatal(e)
			}
			b, e := rewrite.Analyze(q)
			if e != nil {
				t.Fatal(e)
			}
			gracecheck.Equal(t, a, b)
			gracecheck.Check(t, q)
		})
	}
}

func TestInlineCalleeChainDepth(t *testing.T) {
	for _, depth := range []int{0, 1, 2, 3, 4, 64} {
		t.Run(fmt.Sprintf("depth_%d", depth), func(t *testing.T) {
			i := hir.T(hir.I32)
			c := &hir.Class{Name: "Chain"}
			for n := 0; n < 4; n++ {
				x := hir.L(i, 7)
				if n < 3 {
					x = &hir.Expr{Kind: hir.VirtualCall, Name: fmt.Sprintf("m%d", n+1), Type: i, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Chain")}}
				}
				c.Methods = append(c.Methods, &hir.Method{Name: fmt.Sprintf("m%d", n), Virtual: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: x})})
			}
			p := &hir.Program{Classes: []*hir.Class{c}}
			src := strings.ReplaceAll(gracecheck.Source(t, "inline"), "depth 64", fmt.Sprintf("depth %d", depth))
			_, rs, e := rewrite.Parse(src)
			if e != nil {
				t.Fatal(e)
			}
			if _, e := rewrite.Rewrite(p, rs, rewrite.Limits{}); e != nil {
				t.Fatal(e)
			}
			x := c.Methods[0].Body.List[0].X
			if depth < 3 {
				if x.Kind != hir.VirtualCall || x.Name != fmt.Sprintf("m%d", depth+1) {
					t.Fatalf("depth %d: got %s %s", depth, x.Kind, x.Name)
				}
			} else if x.Kind != hir.Lit {
				t.Fatalf("depth %d: chain did not expand to leaf: %s", depth, x.Kind)
			}
		})
	}
}
