package rewrite_test

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
	"testing"
)

func TestFlowLiveness(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	v := func() *hir.Expr { return hir.V("a", arr) }
	read := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: num, X: v()}}
	}
	cases := []struct {
		name string
		body *hir.Stmt
		last bool
	}{
		{"straight", hir.B(read(), read()), false},
		{"last", hir.B(read()), true},
		{"if-then", hir.B(read(), &hir.Stmt{Kind: hir.If, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(read()), Else: hir.B()}), false},
		{"if-else", hir.B(read(), &hir.Stmt{Kind: hir.If, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(), Else: hir.B(read())}), false},
		{"early-return", hir.B(read(), &hir.Stmt{Kind: hir.Return}, read()), true},
		{"kill", hir.B(read(), &hir.Stmt{Kind: hir.Assign, X: v(), Y: &hir.Expr{Kind: hir.New, Type: arr}}, read()), true},
		{"try-finally", hir.B(read(), &hir.Stmt{Kind: hir.Finally, Body: hir.B(), Else: hir.B(read())}), false},
		{"try-catch", hir.B(read(), &hir.Stmt{Kind: hir.Try, Name: "err", Type: num, Body: hir.B(&hir.Stmt{Kind: hir.Throw, Type: num, X: hir.L(num, 1)}), Else: hir.B(read())}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: tc.body}
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
			db, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := db.Has("not_read_after", "C::run/param/a", "C::run/body/s0/x/0"); got != tc.last {
				t.Fatalf("last=%t want %t", got, tc.last)
			}
			gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
		})
	}
}
func TestFlowNestedLoopsAndSeq(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	read := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: num, X: hir.V("a", arr)}}
	}
	inner := &hir.Stmt{Kind: hir.ForEach, Name: "j", Type: num, X: hir.V("a", arr), Body: hir.B(read(), &hir.Stmt{Kind: hir.Continue})}
	outer := &hir.Stmt{Kind: hir.ForEach, Name: "i", Type: num, X: hir.V("a", arr), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.Seq, Type: num, Stmt: hir.B(inner), Y: hir.L(num, 1)}}, &hir.Stmt{Kind: hir.Break})}
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(outer)}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	path := "C::run/body/s0/body/s0/x/seq/s0"
	if !db.Has("in_body", path+"/body/s0", "C::run/body/s0") || !db.Has("in_body", path+"/body/s0", path) {
		t.Fatal("nested membership")
	}
	if db.Has("not_read_after", "C::run/param/a", path+"/body/s0/x/0") {
		t.Fatal("back-edge read lost")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}
func TestWriteSummaries(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	mut := &hir.Method{Name: "mut", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: num, X: hir.V("a", arr), Args: []*hir.Expr{hir.L(num, 1)}}})}
	own := &hir.Method{Name: "own", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: num, Name: "n", X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}, Y: hir.L(num, 1)})}
	caller := &hir.Method{Name: "call", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "b", Type: arr, X: hir.V("a", arr)}, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Owner: "C", Name: "mut", Type: hir.T(hir.Void), Args: []*hir.Expr{hir.V("b", arr)}}})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Fields: []hir.Field{{Name: "n", Type: num}}, Methods: []*hir.Method{mut, own, caller, {Name: "none", Static: true, Result: hir.T(hir.Void), Body: hir.B()}}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range [][2]string{{"C::own", "own_fields"}, {"C::mut", "any"}, {"C::call", "any"}, {"C::none", "none"}} {
		if !db.Has("writes", r[0], r[1]) {
			t.Fatalf("missing writes %v", r)
		}
	}
	if !db.Has("site_writes", "C::call/body/s1/x", "C::call/param/a") {
		t.Fatal("alias write through call lost")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}
