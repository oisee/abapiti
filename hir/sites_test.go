package hir

import (
	"strings"
	"testing"
)

func TestSiteIDsIgnoreSerialsAndSeparateOrdinals(t *testing.T) {
	makeProgram := func(serial int) *Program {
		c := &Class{Name: "src/probe.ts.Probe"}
		c.Methods = []*Method{{Name: "run", Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Node: Node{ID: serial, Source: "src/probe.ts:2:4"}, Kind: RuntimeOp}}, &Stmt{Kind: ExprStmt, X: &Expr{Node: Node{ID: serial + 1, Source: "src/probe.ts:2:4"}, Kind: RuntimeOp}})}}
		return &Program{Classes: []*Class{c}}
	}
	a, b := makeProgram(1), makeProgram(500)
	AssignSiteIDs(a)
	AssignSiteIDs(b)
	x, y := a.Classes[0].Methods[0].Body.List, b.Classes[0].Methods[0].Body.List
	for i := range x {
		if x[i].X.SiteID != y[i].X.SiteID {
			t.Fatal("serial affected site")
		}
	}
	if x[0].X.SiteID == x[1].X.SiteID || !strings.HasSuffix(x[1].X.SiteID, "|1") {
		t.Fatal("missing ordinal")
	}
	first := x[0].X.SiteID
	AssignSiteIDs(a)
	if x[0].X.SiteID != first {
		t.Fatal("not idempotent")
	}
}

func TestInlineNodeChain(t *testing.T) {
	original := Node{SiteID: "leaf", SiteSource: "leaf.ts:1:1", SiteOwner: "Leaf.run", InlinePath: []string{"inner"}}
	call := Node{SiteID: "middle", InlinePath: []string{"outer"}}
	got := InlineNode(original, call, Node{ID: 4, Source: "legacy"})
	if got.SiteID != "leaf" || got.Source != "legacy" || strings.Join(got.InlinePath, "/") != "outer/middle/inner" {
		t.Fatalf("%+v", got)
	}
	got.InlinePath[0] = "changed"
	if call.InlinePath[0] != "outer" {
		t.Fatal("path aliases call")
	}
}
