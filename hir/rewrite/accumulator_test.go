package rewrite_test

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
)

func accumulatorFixture() (*hir.Program, *hir.Method, *hir.Method) {
	n := hir.T(hir.I32)
	a := hir.T(hir.Array, n)
	push := func(name string, e *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: n, X: hir.V(name, a), Args: []*hir.Expr{e}}}
	}
	m := &hir.Method{Name: "m", Virtual: true, Result: a, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "result", Type: a, X: &hir.Expr{Kind: hir.New, Type: a}}, push("result", hir.L(n, 1)), &hir.Stmt{Kind: hir.Return, X: hir.V("result", a)})}
	call := &hir.Expr{Kind: hir.VirtualCall, Name: "m", Type: a, X: hir.V("recv", hir.Ref("C"))}
	caller := &hir.Method{Name: "caller", Static: true, Params: []hir.Param{{Name: "recv", Type: hir.Ref("C")}}, Result: a, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "out", Type: a, X: &hir.Expr{Kind: hir.New, Type: a}}, &hir.Stmt{Kind: hir.ForEach, Name: "x", Type: n, X: call, Body: hir.B(push("out", hir.V("x", n)))}, &hir.Stmt{Kind: hir.Return, X: hir.V("out", a)})}
	return &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m, caller}}}}, m, caller
}
func TestAccumulatorGuards(t *testing.T) {
	cases := []struct {
		name   string
		change func(*hir.Program, *hir.Method, *hir.Method)
		want   int
		guard  string
	}{
		{name: "append", want: 1, guard: "may_throw=false"},
		{name: "result-read", change: func(_ *hir.Program, m, _ *hir.Method) {
			m.Body.List = append(m.Body.List[:1], append([]*hir.Stmt{{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: hir.T(hir.I32), X: hir.V("result", m.Result)}}}, m.Body.List[1:]...)...)
		}},
		{name: "result-alias", change: func(_ *hir.Program, m, _ *hir.Method) {
			m.Body.List = append(m.Body.List[:1], append([]*hir.Stmt{{Kind: hir.VarDecl, Name: "alias", Type: m.Result, X: hir.V("result", m.Result)}}, m.Body.List[1:]...)...)
		}},
		{name: "result-escape", change: func(p *hir.Program, m, _ *hir.Method) {
			p.Classes[0].Fields = []hir.Field{{Name: "saved", Type: m.Result, Static: true}}
			m.Body.List = append(m.Body.List[:1], append([]*hir.Stmt{{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "C", Name: "saved", Type: m.Result}, Y: hir.V("result", m.Result)}}, m.Body.List[1:]...)...)
		}},
		{name: "out-alias", change: func(_ *hir.Program, _, c *hir.Method) {
			c.Body.List = append(c.Body.List[:1], append([]*hir.Stmt{{Kind: hir.VarDecl, Name: "alias", Type: c.Result, X: hir.V("out", c.Result)}}, c.Body.List[1:]...)...)
		}},
		{name: "out-argument", change: func(_ *hir.Program, m, c *hir.Method) {
			m.Params = []hir.Param{{Name: "a", Type: m.Result}}
			c.Body.List[1].X.Args = []*hir.Expr{hir.V("out", c.Result)}
		}},
		{name: "throw-dead", want: 1, guard: "fresh local dead on exception", change: func(_ *hir.Program, m, _ *hir.Method) {
			m.Body.List = append(m.Body.List[:2], append([]*hir.Stmt{{Kind: hir.Throw, X: hir.L(hir.T(hir.I32), 1)}}, m.Body.List[2:]...)...)
		}},
		{name: "throw-live", change: func(_ *hir.Program, m, c *hir.Method) {
			m.Body.List = append(m.Body.List[:2], append([]*hir.Stmt{{Kind: hir.Throw, X: hir.L(hir.T(hir.I32), 1)}}, m.Body.List[2:]...)...)
			c.Params = append(c.Params, hir.Param{Name: "out", Type: c.Result})
			c.Body.List = c.Body.List[1:]
		}},
		{name: "throw-caught", change: func(_ *hir.Program, m, c *hir.Method) {
			m.Body.List = append(m.Body.List[:2], append([]*hir.Stmt{{Kind: hir.Throw, X: hir.L(hir.T(hir.I32), 1)}}, m.Body.List[2:]...)...)
			c.Body.List[1] = &hir.Stmt{Kind: hir.Try, Name: "err", Type: hir.T(hir.I32), Body: hir.B(c.Body.List[1]), Else: hir.B()}
		}},
		{name: "arbitrary-loop", change: func(_ *hir.Program, _, c *hir.Method) {
			c.Body.List[1].Body.List = append(c.Body.List[1].Body.List, &hir.Stmt{Kind: hir.ExprStmt, X: hir.L(hir.T(hir.I32), 42)})
		}},
		{name: "temporary", want: 1, guard: "may_throw=false", change: func(_ *hir.Program, _, c *hir.Method) {
			loop := c.Body.List[1]
			decl := &hir.Stmt{Kind: hir.VarDecl, Name: "temp", Type: c.Result, X: loop.X}
			loop.X = hir.V("temp", c.Result)
			c.Body.List = append(c.Body.List[:1], append([]*hir.Stmt{decl}, c.Body.List[1:]...)...)
		}},
		{name: "out-read-between", change: func(_ *hir.Program, _, c *hir.Method) {
			loop := c.Body.List[1]
			decl := &hir.Stmt{Kind: hir.VarDecl, Name: "temp", Type: c.Result, X: loop.X}
			loop.X = hir.V("temp", c.Result)
			read := &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: hir.T(hir.I32), X: hir.V("out", c.Result)}}
			c.Body.List = append(c.Body.List[:1], append([]*hir.Stmt{decl, read}, c.Body.List[1:]...)...)
		}},
		{name: "consumed-twice", change: func(_ *hir.Program, _, c *hir.Method) {
			loop := c.Body.List[1]
			decl := &hir.Stmt{Kind: hir.VarDecl, Name: "temp", Type: c.Result, X: loop.X}
			loop.X = hir.V("temp", c.Result)
			read := &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: hir.T(hir.I32), X: hir.V("temp", c.Result)}}
			c.Body.List = []*hir.Stmt{c.Body.List[0], decl, loop, read, c.Body.List[2]}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, m, c := accumulatorFixture()
			if tc.change != nil {
				tc.change(p, m, c)
			}
			db, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
			st, err := rewrite.Accumulator(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(st.Sites) != tc.want {
				t.Fatalf("sites=%v want %d", st, tc.want)
			}
			if tc.want > 0 && st.Sites[0].Guard != tc.guard {
				t.Fatal(st.Sites)
			}
			if es := hir.Verify(p); len(es) > 0 {
				t.Fatal(es)
			}
		})
	}
}
func TestAccumulatorPolymorphicDefaults(t *testing.T) {
	p, m, c := accumulatorFixture()
	a := m.Result
	p.Interfaces = []*hir.Interface{{Name: "I", Methods: []*hir.Method{{Name: "m", Virtual: true, Abstract: true, Result: a}}}}
	p.Classes[0].Implements = []string{"I"}
	p.Classes = append(p.Classes, &hir.Class{Name: "D", Implements: []string{"I"}, Methods: []*hir.Method{{Name: "m", Virtual: true, Result: a, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: a}})}}})
	c.Params[0].Type = hir.Type{Kind: hir.InterfaceRef, Name: "I"}
	c.Body.List[1].X.X.Type = c.Params[0].Type
	st, err := rewrite.Accumulator(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Clones != 1 || st.Defaults != 1 || len(st.Sites) != 1 {
		t.Fatal(st)
	}
	if p.Interfaces[0].Methods[1].Abstract {
		t.Fatal("interface variant ABSTRACT")
	}
	def := p.Classes[1].Methods[1]
	if def.Body.List[0].X.Name != "m" {
		t.Fatal("recursive forwarding default")
	}
	again, err := rewrite.Accumulator(p)
	if err != nil || len(again.Sites) != 0 {
		t.Fatalf("not idempotent: %v %v", again, err)
	}
	if !strings.HasSuffix(p.Classes[0].Methods[2].Name, "_into") {
		t.Fatal("missing variant")
	}
}

func TestAccumulatorFrontendTemporary(t *testing.T) {
	p, m, c := accumulatorFixture()
	for _, method := range []*hir.Method{m, c} {
		decl := method.Body.List[0]
		temp := &hir.Stmt{Kind: hir.VarDecl, Name: "tmp", Type: decl.Type, X: decl.X}
		decl.X = hir.V("tmp", decl.Type)
		method.Body.List = append([]*hir.Stmt{temp}, method.Body.List...)
	}
	st, err := rewrite.Accumulator(p)
	if err != nil || st.Clones != 1 || len(st.Sites) != 1 {
		t.Fatalf("%v %v", st, err)
	}
	if len(p.Classes[0].Methods[2].Body.List) != 2 {
		t.Fatal("allocation prefix retained")
	}
}

func TestAccumulatorTemporaryEscape(t *testing.T) {
	p, _, c := accumulatorFixture()
	decl := c.Body.List[0]
	temp := &hir.Stmt{Kind: hir.VarDecl, Name: "tmp", Type: decl.Type, X: decl.X}
	decl.X = hir.V("tmp", decl.Type)
	p.Classes[0].Fields = []hir.Field{{Name: "saved", Static: true, Type: decl.Type}}
	escape := &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "C", Name: "saved", Type: decl.Type}, Y: hir.V("tmp", decl.Type)}
	c.Body.List = append([]*hir.Stmt{temp, escape}, c.Body.List...)
	st, err := rewrite.Accumulator(p)
	if err != nil || len(st.Sites) != 0 {
		t.Fatalf("%v %v", st, err)
	}
}

func TestAccumulatorSeparateSignatures(t *testing.T) {
	p, _, _ := accumulatorFixture()
	p.Classes = append(p.Classes, &hir.Class{Name: "Unrelated", Methods: []*hir.Method{{Name: "m", Result: hir.T(hir.I32), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(hir.T(hir.I32), 7)})}}})
	st, err := rewrite.Accumulator(p)
	if err != nil || len(st.Sites) != 1 {
		t.Fatalf("%v %v", st, err)
	}
	if len(p.Classes[1].Methods) != 1 {
		t.Fatal("unrelated signature changed")
	}
}

func TestAccumulatorWholeAppendFastPath(t *testing.T) {
	for _, extra := range []bool{false, true} {
		t.Run(map[bool]string{false: "whole-append", true: "additional-read"}[extra], func(t *testing.T) {
			p, _, c := accumulatorFixture()
			loop := c.Body.List[1]
			call := loop.X
			loop.X = hir.V("temp", c.Result)
			decl := &hir.Stmt{Kind: hir.VarDecl, Name: "temp", Type: c.Result, X: call}
			n := c.Result.Args[0]
			branch := &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: n, X: hir.V("temp", c.Result)}, Y: hir.L(n, 1)}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: n, X: hir.V("out", c.Result), Args: []*hir.Expr{{Kind: hir.IndexGet, Type: n, X: hir.V("temp", c.Result), Y: hir.L(n, 0)}}}}), Else: hir.B(loop)}
			c.Body.List = []*hir.Stmt{c.Body.List[0], decl, branch, c.Body.List[2]}
			if extra {
				c.Body.List = append(c.Body.List[:3], append([]*hir.Stmt{{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: n, X: hir.V("temp", c.Result)}}}, c.Body.List[3:]...)...)
			}
			db, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
			st, err := rewrite.Accumulator(p)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if extra {
				want = 0
			}
			if len(st.Sites) != want {
				t.Fatal(st)
			}
		})
	}
}
