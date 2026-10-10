package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

var ints = hir.T(hir.Array, hir.T(hir.I32))

func local(name string) *hir.Expr { return hir.V(name, ints) }
func each(source *hir.Expr, body *hir.Stmt) *hir.Stmt {
	return &hir.Stmt{Kind: hir.ForEach, Name: "e", Type: hir.T(hir.I32), X: source, Body: body}
}

func TestLiteralProvenance(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "source.ts")
	for _, tt := range []struct {
		text string
		yes  bool
	}{{"[x]", true}, {"[]", false}, {"[\n ]", false}, {"[/*comment*/x]", true}, {"[/*comment*/]", false}, {"[...xs]", false}, {"new Array(1)", false}} {
		if err := os.WriteFile(file, []byte(tt.text), 0644); err != nil {
			t.Fatal(err)
		}
		if got := literalSource(file + ":1:1"); got != tt.yes {
			t.Fatalf("%q: %t", tt.text, got)
		}
	}
}
func TestParameterUse(t *testing.T) {
	tests := []struct {
		name    string
		body    *hir.Stmt
		yes     bool
		blocker string
	}{
		{"foreach", hir.B(each(local("r"), hir.B(&hir.Stmt{Kind: hir.Return, X: hir.V("e", hir.T(hir.I32))}))), true, ""},
		{"unused", hir.B(), true, ""},
		{"return", hir.B(&hir.Stmt{Kind: hir.Return, X: local("r")}), false, "return"},
		{"alias", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: ints, X: local("r")}), false, "var"},
		{"length", hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: hir.T(hir.I32), X: local("r")}}), false, "array.length"},
		{"index", hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.IndexGet, Type: hir.T(hir.I32), X: local("r"), Y: hir.L(hir.T(hir.I32), 0)}}), false, "index"},
		{"forward", hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Args: []*hir.Expr{local("r")}}}), false, "call argument"},
		{"stored", hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet}, Y: local("r")}), false, "assign value"},
		{"cast foreach", hir.B(each(&hir.Expr{Kind: hir.Cast, Type: ints, X: local("r")}, hir.B())), false, "cast"},
		{"shadow", hir.B(each(local("r"), hir.B()), hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "r", Type: ints, X: &hir.Expr{Kind: hir.New, Type: ints}}, &hir.Stmt{Kind: hir.Return, X: local("r")})), true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &hir.Method{Name: "run", Params: []hir.Param{{Name: "r", Type: ints}}, Body: tt.body}
			u := parameterUse("C::run", m, 0, rewrite.NewDB())
			if u.Eligible != tt.yes || !strings.Contains(u.Blocker, tt.blocker) {
				t.Fatalf("%+v", u)
			}
		})
	}
}

func TestSingletonReceiversAndChain(t *testing.T) {
	iface := hir.Type{Kind: hir.InterfaceRef, Name: "I"}
	params := []hir.Param{{Name: "r", Type: ints}}
	good := &hir.Method{Name: "run", Virtual: true, Params: params, Result: ints, Body: hir.B(each(local("r"), hir.B()), &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: ints}})}
	bad := &hir.Method{Name: "run", Virtual: true, Params: params, Result: ints, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: local("r")})}
	makeCall := func(arg *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.VirtualCall, Name: "run", Type: ints, X: hir.V("runner", iface), Args: []*hir.Expr{arg}}
	}
	fresh := &hir.Stmt{Kind: hir.VarDecl, Name: "t", Type: ints, X: &hir.Expr{Kind: hir.New, Type: ints}}
	push := &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: local("t"), Args: []*hir.Expr{hir.L(hir.T(hir.I32), 1)}}}
	alias := &hir.Stmt{Kind: hir.VarDecl, Name: "temp", Type: ints, X: local("t")}
	chain := each(hir.V("list", hir.T(hir.Array, hir.T(hir.I32))), hir.B(&hir.Stmt{Kind: hir.Assign, X: local("temp"), Y: makeCall(local("temp"))}))
	caller := &hir.Method{Name: "caller", Static: true, Params: []hir.Param{{Name: "runner", Type: iface}, {Name: "list", Type: hir.T(hir.Array, hir.T(hir.I32))}}, Result: hir.T(hir.Void), Body: hir.B(fresh, push, alias, chain)}
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: "I", Methods: []*hir.Method{{Name: "run", Abstract: true, Params: params, Result: ints}}}}, Classes: []*hir.Class{{Name: "Good", Implements: []string{"I"}, Methods: []*hir.Method{good}}, {Name: "Bad", Implements: []string{"I"}, Methods: []*hir.Method{bad}}, {Name: "Caller", Methods: []*hir.Method{caller}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	sites, _ := count(p, db, nil)
	if len(sites) != 2 {
		t.Fatalf("sites: %+v", sites)
	}
	for _, s := range sites {
		if s.Verdict != "partially" || s.Yes != 1 || len(s.Targets) != 2 {
			t.Fatalf("%+v", s)
		}
	}
	if sites[0].Form != "chained" || sites[1].Form != "local-first" {
		t.Fatalf("%+v", sites)
	}
	// Production lowering wraps literal construction in Seq, yielding a Local.
	seq := &hir.Expr{Kind: hir.Seq, Type: ints, Stmt: hir.B(fresh, push), Y: local("t")}
	alias.X = seq
	caller.Body = hir.B(alias, chain)
	db, err = rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	sites, _ = count(p, db, nil)
	if len(sites) != 2 || sites[1].Form != "local-first" {
		t.Fatalf("Seq local chain: %+v", sites)
	}
	caller.Body = hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: makeCall(seq)})
	db, err = rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	sites, _ = count(p, db, nil)
	if len(sites) != 1 || sites[0].Form != "literal" {
		t.Fatalf("Seq literal: %+v", sites)
	}
	alias.X = local("t")
	// A second push is not a singleton; a prior escaping call is not proven S1.
	caller.Body = hir.B(fresh, push, push, alias, chain)
	db, err = rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	sites, _ = count(p, db, nil)
	if len(sites) != 0 {
		t.Fatalf("multi-element: %+v", sites)
	}
	caller.Body = hir.B(fresh, push, &hir.Stmt{Kind: hir.ExprStmt, X: makeCall(local("t"))}, alias, chain)
	db, err = rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	sites, _ = count(p, db, nil)
	if len(sites) != 1 || sites[0].Form != "literal" {
		t.Fatalf("prior call: %+v", sites)
	}
}
