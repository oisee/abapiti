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

func TestSyntheticSitesUseOwnerKindOrdinal(t *testing.T) {
	a := &Expr{Node: Node{Source: "synthetic.one"}, Kind: New}
	b := &Expr{Node: Node{Source: "synthetic.two"}, Kind: New}
	p := &Program{Classes: []*Class{{Name: "C", Methods: []*Method{{Name: "run", Body: B(&Stmt{Kind: ExprStmt, X: a}, &Stmt{Kind: ExprStmt, X: b})}}}}}
	AssignSiteIDs(p)
	if a.SiteID != "C.run||new|0" || b.SiteID != "C.run||new|1" {
		t.Fatalf("%s %s", a.SiteID, b.SiteID)
	}
}

func TestSharedBridgeSitesBelongToEachMethod(t *testing.T) {
	shared := &Expr{Node: Node{ID: 7, Source: "probe.ts:3:4"}, Kind: VirtualCall}
	body := B(&Stmt{Kind: ExprStmt, X: shared}, &Stmt{Kind: ExprStmt, X: shared})
	a, b := &Method{Name: "set_value", Body: body}, &Method{Name: "set", Body: body}
	p := &Program{Classes: []*Class{{Name: "probe.ts.C", Methods: []*Method{a, b}}}}
	before := Dump(p)
	AssignSiteIDs(p)
	x, y := a.Body.List[0].X, b.Body.List[0].X
	if x == y || x.SiteID == y.SiteID || x.SiteOwner != "probe.ts.C.set_value" || y.SiteOwner != "probe.ts.C.set" {
		t.Fatalf("shared owner: %+v %+v", x.Node, y.Node)
	}
	if a.Body.List[1].X != x || b.Body.List[1].X != y {
		t.Fatal("within-method sharing changed")
	}
	if Dump(p) != before || x.ID != 7 || y.ID != 7 {
		t.Fatal("legacy IR changed")
	}
	first, second := x.SiteID, y.SiteID
	AssignSiteIDs(p)
	if x.SiteID != first || y.SiteID != second {
		t.Fatal("detachment not idempotent")
	}
}
