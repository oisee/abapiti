package main

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"testing"
)

func TestFlowCandidates(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	v := func(n string) *hir.Expr { return hir.V(n, arr) }
	fresh := func() *hir.Expr { return &hir.Expr{Kind: hir.New, Type: arr} }
	loop := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ForEach, Name: "row", Type: num, X: v("a"), Body: hir.B()}
	}
	copyArray := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.VarDecl, Name: "b", Type: arr, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.slice0", Type: arr, X: v("a")}}
	}
	cases := []struct {
		name string
		body *hir.Stmt
		rule string
		yes  bool
	}{
		{"r1", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, loop()), "R1", true},
		{"r1-mutation", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, &hir.Stmt{Kind: hir.ForEach, Name: "row", Type: num, X: v("a"), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: num, X: v("a"), Args: []*hir.Expr{hir.L(num, 1)}}})}), "R1", false},
		{"r2", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, loop()), "R2", true},
		{"r2-double", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, loop(), loop()), "R2", false},
		{"r3", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, copyArray()), "R3", true},
		{"r3-later", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, copyArray(), loop()), "R3", false},
		{"r3-alias", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, &hir.Stmt{Kind: hir.VarDecl, Name: "alias", Type: arr, X: v("a")}, copyArray()), "R3", false},
		{"r3-double-operand", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: fresh()}, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.concat", Type: arr, X: v("a"), Args: []*hir.Expr{v("a")}}}), "R3", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.Void), Body: tc.body}}}}}
			db, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			ss := flowCounts(p, db, "")
			found := false
			for _, s := range ss {
				if s.Rule == tc.rule {
					found = true
					if (s.Verdict == "qualifies") != tc.yes {
						t.Fatalf("%+v want eligible=%t", s, tc.yes)
					}
				}
			}
			if !found {
				t.Fatalf("no %s sites", tc.rule)
			}
		})
	}
}

func TestAppendValueMoveCandidates(t *testing.T) {
	str := hir.T(hir.String)
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, str)
	for _, later := range []bool{false, true} {
		t.Run(fmt.Sprint(later), func(t *testing.T) {
			body := hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "out", Type: arr, X: &hir.Expr{Kind: hir.New, Type: arr}}, &hir.Stmt{Kind: hir.VarDecl, Name: "s", Type: str, X: hir.L(str, "value")}, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: num, X: hir.V("out", arr), Args: []*hir.Expr{hir.V("s", str)}}})
			if later {
				body.List = append(body.List, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.length", Type: num, X: hir.V("s", str)}})
			}
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.Void), Body: body}}}}}
			db, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, s := range flowCounts(p, db, "") {
				if s.Rule == "R3" {
					found = true
					if (s.Verdict == "qualifies") == later {
						t.Fatalf("%+v", s)
					}
				}
			}
			if !found {
				t.Fatal("missing append copy")
			}
		})
	}
}
