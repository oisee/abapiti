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
			q := gracecheck.Clone(t, p)
			r := rand.New(rand.NewSource(seed + 1000))
			r.Shuffle(len(q.Classes), func(i, j int) { q.Classes[i], q.Classes[j] = q.Classes[j], q.Classes[i] })
			for _, c := range q.Classes {
				r.Shuffle(len(c.Methods), func(i, j int) { c.Methods[i], c.Methods[j] = c.Methods[j], c.Methods[i] })
			}
			a, e := rewrite.Analyze(p)
			if e != nil {
				t.Fatal(e)
			}
			b, e := rewrite.Analyze(q)
			if e != nil {
				t.Fatal(e)
			}
			gracecheck.Equal(t, a, b)
			// Positive subprograms are monotone. Negated complements (e.g. pure) are
			// intentionally excluded: adding writes can retract purity on a fresh run.
			src := gracecheck.Source(t, "analysis")
			base := rewrite.Extract(p)
			positiveMonotonicity(t, base, src)
		})
	}
}
func positiveMonotonicity(t *testing.T, base *rewrite.DB, src string) {
	t.Helper() // Keep whole rules only when every alternative is positive.
	// The transitive throw/write/escape rules are the monotone recursive kernel.
	src = `(rule throw 0 (head (may_throw ?m)) (base (throws ?m _)) (tail (calls ?m ?n _) (may_throw ?n)))
 (rule write 0 (head (writes_static_transitive ?m ?c ?f)) (base (writes_static ?m ?c ?f)) (tail (calls ?m ?n _) (writes_static_transitive ?n ?c ?f)))`
	before := gracecheck.Evaluate(t, base, src)
	for _, tuple := range base.Facts("defined") {
		if e := base.Add("throws", tuple[0], "added"); e != nil {
			t.Fatal(e)
		}
		if e := base.Add("writes_static", tuple[0], "Graph", "extra"); e != nil {
			t.Fatal(e)
		}
	}
	after := gracecheck.Evaluate(t, base, src)
	for _, p := range before.Predicates() {
		for _, a := range before.Facts(p) {
			if !after.Has(p, a...) {
				t.Fatalf("adding facts retracted %s%v", p, a)
			}
		}
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
