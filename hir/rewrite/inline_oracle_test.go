package rewrite_test

import (
	"os"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
	"github.com/oisee/abapiti/internal/inlineoracle"
)

func inlineFixture() *hir.Program {
	i := hir.T(hir.I32)
	v := hir.T(hir.Void)
	bo := hir.T(hir.Bool)
	ref := hir.Ref("C")
	lit := func(n int) *hir.Expr { return hir.L(i, n) }
	self := func() *hir.Expr { return &hir.Expr{Kind: hir.This, Type: ref} }
	field := func() *hir.Expr { return &hir.Expr{Kind: hir.FieldGet, Type: i, Name: "n", X: self()} }
	ret := func(x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: x} }
	vc := func(receiver *hir.Expr, name string, result hir.Type, args ...*hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.VirtualCall, X: receiver, Name: name, Type: result, Args: args}
	}
	met := func(name string, result hir.Type, body ...*hir.Stmt) *hir.Method {
		return &hir.Method{Name: name, Virtual: true, Result: result, Body: hir.B(body...)}
	}
	getter := met("get", i, ret(field()))
	bump := met("bump", v, &hir.Stmt{Kind: hir.Assign, X: field(), Y: lit(9)}, &hir.Stmt{Kind: hir.Return})
	pair := met("pair", i, ret(&hir.Expr{Kind: hir.Binary, Op: "+", Type: i, X: hir.V("a", i), Y: hir.V("b", i)}))
	pair.Params = []hir.Param{{Name: "a", Type: i}, {Name: "b", Type: i}}
	early := met("early", i, &hir.Stmt{Kind: hir.If, X: hir.L(bo, true), Body: hir.B(ret(lit(7)))}, ret(vc(self(), "get", i)))
	recurse := met("cycle", i, ret(vc(self(), "cycle", i)))
	shadow := met("shadow", i, hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "x", Type: i, X: lit(1)}), hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "x", Type: i, X: lit(2)}), ret(lit(3)))
	obj := func() *hir.Expr { return hir.V("obj", ref) }
	run := &hir.Method{Name: "run", Static: true, Result: i, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "obj", Type: ref, X: &hir.Expr{Kind: hir.New, Name: "C", Type: ref}},
		&hir.Stmt{Kind: hir.ExprStmt, X: vc(obj(), "bump", v)},
		&hir.Stmt{Kind: hir.ExprStmt, X: vc(obj(), "cycle", i)},
		&hir.Stmt{Kind: hir.ExprStmt, X: vc(obj(), "shadow", i)},
		&hir.Stmt{Kind: hir.ExprStmt, X: vc(obj(), "early", i)},
		ret(vc(obj(), "pair", i, vc(obj(), "get", i), lit(2))),
	)}
	return &hir.Program{Classes: []*hir.Class{{Name: "C", Fields: []hir.Field{{Name: "n", Type: i}}, Methods: []*hir.Method{getter, bump, pair, early, recurse, shadow, run}}}}
}
func TestInlineActionOracle(t *testing.T) {
	p := inlineFixture()
	inlineoracle.Check(t, p)
	s, e := rewrite.Inline(p)
	if e != nil {
		t.Fatal(e)
	}
	if s.CallSites != 5 {
		t.Fatalf("want 5 sites, got %+v", s)
	}
}
func TestInlineBudgetAndDepth(t *testing.T) {
	src, e := os.ReadFile("rules/inline.grace")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		source string
		limits rewrite.Limits
		max    int
	}{
		{"method growth", string(src), rewrite.Limits{MethodGrowth: 1}, 1},
		{"program growth", string(src), rewrite.Limits{ProgramGrowth: 1}, 1},
		{"zero depth", strings.ReplaceAll(string(src), "depth 64", "depth 0"), rewrite.Limits{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, rs, e := rewrite.Parse(tc.source)
			if e != nil {
				t.Fatal(e)
			}
			p := inlineFixture()
			s, e := rewrite.Rewrite(p, rs, tc.limits)
			if e != nil {
				t.Fatal(e)
			}
			if s.CallSites > tc.max {
				t.Fatalf("limit ignored: %+v", s)
			}
		})
	}
}

func TestInlineSelectionOracle(t *testing.T) {
	i := hir.T(hir.I32)
	ret := func(n int) *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: hir.L(i, n)} }
	for _, tc := range []struct {
		name   string
		change func(*hir.Program)
	}{
		{"prefix collision", func(p *hir.Program) { p.Classes[0].Methods[6].Params = []hir.Param{{Name: "inl_existing", Type: i}} }},
		{"assigned literal parameter", func(p *hir.Program) {
			m := p.Classes[0].Methods[2]
			m.Body.List = append([]*hir.Stmt{{Kind: hir.Assign, X: hir.V("b", i), Y: hir.L(i, 3)}}, m.Body.List...)
		}},
		{"checker variant", func(p *hir.Program) {
			p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "get_instantiated_number", Virtual: true, Result: i, Body: hir.B(ret(4))})
		}},
		{"descendant override", func(p *hir.Program) {
			p.Classes = append(p.Classes, &hir.Class{Name: "D", Super: "C", Methods: []*hir.Method{{Name: "get", Virtual: true, Result: i, Body: hir.B(ret(4))}}})
		}},
		{"large callee", func(p *hir.Program) {
			m := p.Classes[0].Methods[2]
			for n := 0; n < 12; n++ {
				m.Body.List = append([]*hir.Stmt{{Kind: hir.ExprStmt, X: hir.L(i, n)}}, m.Body.List...)
			}
		}},
		{"mutual recursion", func(p *hir.Program) {
			m := p.Classes[0].Methods[4]
			m.Body.List[0].X.Name = "other"
			p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "other", Virtual: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "cycle", Type: i, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}})})
		}},
		{"Seq lexical scope", func(p *hir.Program) {
			m := p.Classes[0].Methods[0]
			m.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Seq, Type: i, Stmt: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "tmp", Type: i, X: hir.L(i, 6)}), Y: hir.V("tmp", i)}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { p := inlineFixture(); tc.change(p); inlineoracle.Check(t, p) })
	}
}

// Minimal reproducer: a parameter binding creates a Seq with an enclosing
// block. Omitting that block charges one added node although growth is two.
func TestInlineBudgetCountsSeqBlock(t *testing.T) {
	i := hir.T(hir.I32)
	leaf := &hir.Method{Name: "leaf", Virtual: true, Params: []hir.Param{{Name: "p", Type: i}}, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.V("p", i)})}
	caller := &hir.Method{Name: "caller", Virtual: true, Params: []hir.Param{{Name: "x", Type: i}}, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "leaf", Type: i, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}, Args: []*hir.Expr{hir.V("x", i)}}})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{leaf, caller}}}}
	src, e := os.ReadFile("rules/inline.grace")
	if e != nil {
		t.Fatal(e)
	}
	_, rs, e := rewrite.Parse(string(src))
	if e != nil {
		t.Fatal(e)
	}
	before := hir.Dump(p)
	stats, e := rewrite.Rewrite(p, rs, rewrite.Limits{MethodGrowth: 1, ProgramGrowth: 1})
	if e != nil {
		t.Fatal(e)
	}
	if stats.CallSites != 0 || hir.Dump(p) != before {
		t.Fatalf("Seq enclosing block escaped budget: %+v", stats)
	}
	stats, e = rewrite.Rewrite(p, rs, rewrite.Limits{MethodGrowth: 2, ProgramGrowth: 2})
	if e != nil {
		t.Fatal(e)
	}
	if stats.CallSites != 1 {
		t.Fatalf("exact two-node budget rejected: %+v", stats)
	}
}

// Traversal-numbered names are part of the pinned byte-level oracle contract.
// Declaration-order invariance therefore needs alpha-normalisation; changing
// native naming to remove this difference would break milestone 2 compatibility.
func TestInlineOracleNamesFollowDeclarationOrder(t *testing.T) {
	i := hir.T(hir.I32)
	leaf := &hir.Method{Name: "leaf", Virtual: true, Params: []hir.Param{{Name: "p", Type: i}}, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.V("p", i)})}
	caller := func(name string) *hir.Method {
		return &hir.Method{Name: name, Virtual: true, Params: []hir.Param{{Name: "x", Type: i}}, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "leaf", Type: i, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}, Args: []*hir.Expr{hir.V("x", i)}}})}
	}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{leaf, caller("a"), caller("b")}}}}
	q := gracecheck.Clone(t, p)
	q.Classes[0].Methods[1], q.Classes[0].Methods[2] = q.Classes[0].Methods[2], q.Classes[0].Methods[1]
	inlineoracle.Check(t, p)
	inlineoracle.Check(t, q)
	gracecheck.Permutations(t, p)
	if _, e := rewrite.Inline(p); e != nil {
		t.Fatal(e)
	}
	if _, e := rewrite.Inline(q); e != nil {
		t.Fatal(e)
	}
	if gracecheck.CanonicalDump(p) == gracecheck.CanonicalDump(q) {
		t.Fatal("fixture no longer demonstrates pinned traversal naming; revisit determinism contract")
	}
}

// Verified HIR can contain nil block entries. Template extraction visits even
// large methods which the pinned inliner rejects before building a template.
func TestInlineFactsNilStatementInRejectedMethod(t *testing.T) {
	i := hir.T(hir.I32)
	ret := func() *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: hir.L(i, 1)} }
	for _, body := range []*hir.Stmt{
		hir.B(nil, ret()),
		hir.B(&hir.Stmt{Kind: hir.If, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(ret()), Else: hir.B(nil)}, ret()),
	} {
		// Force the oracle's size rejection while preserving the problematic shape.
		for n := 0; n < 13; n++ {
			body.List = append(body.List, &hir.Stmt{Kind: hir.ExprStmt, X: hir.L(i, n)})
		}
		p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "large", Virtual: true, Result: i, Body: body}}}}}
		if errs := hir.Verify(p); len(errs) != 0 {
			t.Fatal(errs)
		}
		before := hir.Dump(p)
		if _, err := rewrite.ExtractRewriteFacts(p); err != nil {
			t.Fatal(err)
		}
		if hir.Dump(p) != before {
			t.Fatal("template extraction changed HIR")
		}
		inlineoracle.Check(t, p)
	}
}
