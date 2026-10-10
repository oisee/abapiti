package main

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestArrayAliasesRowsAndViews(t *testing.T) {
	base := hir.Ref("Base")
	sub := hir.Ref("Sub")
	arr := hir.T(hir.Array, base)
	subs := hir.T(hir.Array, sub)
	n := &hir.Expr{Kind: hir.New, Type: arr}
	idx := func() *hir.Expr {
		return &hir.Expr{Kind: hir.IndexGet, Type: base, X: hir.V("a", arr), Y: hir.L(hir.T(hir.I32), 0)}
	}
	method := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: n},
		&hir.Stmt{Kind: hir.VarDecl, Name: "alias", Type: arr, X: hir.V("a", arr)},
		&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("alias", arr), Args: []*hir.Expr{{Kind: hir.New, Type: sub}}}},
		&hir.Stmt{Kind: hir.Assign, X: idx(), Y: &hir.Expr{Kind: hir.New, Type: base}},
		&hir.Stmt{Kind: hir.ForEach, Name: "row", Type: base, X: hir.V("a", arr), Body: hir.B()},
		&hir.Stmt{Kind: hir.ForEach, Name: "row", Type: sub, X: &hir.Expr{Kind: hir.Cast, Type: subs, X: hir.V("a", arr)}, Body: hir.B()},
		&hir.Stmt{Kind: hir.ExprStmt, X: idx()},
	)}
	a := &arrayAnalysis{g: newArrayGraph()}
	a.scan(&hir.Program{Classes: []*hir.Class{{Name: "Base"}, {Name: "Sub", Super: "Base"}, {Name: "Caller", Methods: []*hir.Method{method}}}}, "")
	reads := 0
	for _, s := range a.sites {
		if s.Kind == "new" {
			continue
		}
		reads++
		if s.Casts != 1 || s.Removed != 1 {
			t.Fatalf("cast census: %+v", s)
		}
		if s.Proof != "hierarchy/mixed upper bound" || !strings.Contains(s.Classes, "class:Sub") || !strings.Contains(s.Classes, "class:Base") {
			t.Fatalf("row flow: %+v", s)
		}
		if !strings.Contains(s.Views, "classref<Base>") || !strings.Contains(s.Views, "classref<Sub>") {
			t.Fatalf("covariant view: %+v", s)
		}
	}
	if reads != 3 {
		t.Fatalf("index writes must not count as reads: %d", reads)
	}
}
func TestArrayExactAndUnknownProducer(t *testing.T) {
	ref := hir.Ref("Row")
	arr := hir.T(hir.Array, ref)
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: &hir.Expr{Kind: hir.New, Type: arr}},
		&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("a", arr), Args: []*hir.Expr{{Kind: hir.New, Type: ref}}}},
		&hir.Stmt{Kind: hir.ForEach, Name: "r", Type: ref, X: hir.V("a", arr), Body: hir.B()},
		&hir.Stmt{Kind: hir.ForEach, Name: "r", Type: ref, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.values", Type: arr, X: hir.V("map", hir.T(hir.OrderedMap, hir.T(hir.String), ref))}, Body: hir.B()},
	)}
	a := &arrayAnalysis{g: newArrayGraph()}
	a.scan(&hir.Program{Classes: []*hir.Class{{Name: "Row"}, {Name: "Caller", Methods: []*hir.Method{m}}}}, "")
	proofs := map[string]bool{}
	for _, s := range a.sites {
		if s.Kind == "foreach" {
			proofs[s.Proof] = true
		}
	}
	if !proofs["exact"] || !proofs["unknown native/unresolved flow"] {
		t.Fatalf("proofs: %v", proofs)
	}
}
func TestArrayNarrowReadRetainsCast(t *testing.T) {
	arr := hir.T(hir.Array, hir.Ref("Base"))
	m := &hir.Method{Name: "run", Static: true, Body: hir.B(&hir.Stmt{Kind: hir.ForEach, Name: "r", Type: hir.Ref("Sub"), X: &hir.Expr{Kind: hir.New, Type: arr}, Body: hir.B()})}
	a := &arrayAnalysis{g: newArrayGraph()}
	a.scan(&hir.Program{Classes: []*hir.Class{{Name: "Base"}, {Name: "Sub", Super: "Base"}, {Name: "C", Methods: []*hir.Method{m}}}}, "")
	for _, s := range a.sites {
		if s.Kind == "foreach" && (s.Casts != 1 || s.Removed != 0) {
			t.Fatalf("narrow read: %+v", s)
		}
	}
}

func TestArrayInterfaceDispatchDoesNotMixUnrelatedRunMethods(t *testing.T) {
	row := hir.Ref("Row")
	arr := hir.T(hir.Array, row)
	other := hir.T(hir.Array, hir.Ref("Other"))
	iface := hir.Type{Kind: hir.InterfaceRef, Name: "IRunner"}
	impl := &hir.Method{Name: "run", Result: arr, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr, X: &hir.Expr{Kind: hir.New, Type: arr}}, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("a", arr), Args: []*hir.Expr{{Kind: hir.New, Type: row}}}}, &hir.Stmt{Kind: hir.Return, X: hir.V("a", arr)})}
	caller := &hir.Method{Name: "call", Static: true, Params: []hir.Param{{Name: "runner", Type: iface}}, Body: hir.B(&hir.Stmt{Kind: hir.ForEach, Name: "r", Type: row, X: &hir.Expr{Kind: hir.VirtualCall, Name: "run", Type: arr, X: hir.V("runner", iface)}, Body: hir.B()})}
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: "IRunner", Methods: []*hir.Method{{Name: "run", Result: arr}}}}, Classes: []*hir.Class{{Name: "Row"}, {Name: "Other"}, {Name: "Runner", Implements: []string{"IRunner"}, Methods: []*hir.Method{impl}}, {Name: "Unrelated", Methods: []*hir.Method{{Name: "run", Result: other, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: other}})}}}, {Name: "Caller", Methods: []*hir.Method{caller}}}}
	a := &arrayAnalysis{g: newArrayGraph()}
	a.scan(p, "")
	for _, s := range a.sites {
		if s.Kind == "foreach" && (s.Proof != "exact" || s.Classes != "class:Row" || s.Compatible != 1) {
			t.Fatalf("unrelated dispatch polluted rows: %+v", s)
		}
	}
}
