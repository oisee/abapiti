package rewrite_test

import (
	"reflect"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

func TestBothInlinersPreserveSiteChain(t *testing.T) {
	for _, classic := range []bool{false, true} {
		t.Run(map[bool]string{false: "Grace", true: "classic"}[classic], func(t *testing.T) {
			ref := hir.Ref("probe.ts.C")
			allocation := &hir.Expr{Node: hir.Node{Source: "probe.ts:1:1"}, Kind: hir.New, Type: ref}
			self := func() *hir.Expr { return &hir.Expr{Kind: hir.This, Type: ref} }
			call := func(name, source string) *hir.Expr {
				return &hir.Expr{Node: hir.Node{Source: source}, Kind: hir.VirtualCall, Type: ref, X: self(), Name: name}
			}
			middleCall, outerCall := call("leaf", "probe.ts:2:1"), call("middle", "probe.ts:3:1")
			method := func(name string, x *hir.Expr) *hir.Method {
				return &hir.Method{Name: name, Virtual: true, Result: ref, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: x})}
			}
			leaf, middle, outer := method("leaf", allocation), method("middle", middleCall), method("outer", outerCall)
			p := &hir.Program{Classes: []*hir.Class{{Name: ref.Name, Methods: []*hir.Method{leaf, middle, outer}}}}
			hir.AssignSiteIDs(p)
			wantID, source, owner := allocation.SiteID, allocation.SiteSource, allocation.SiteOwner
			wantPath := []string{outerCall.SiteID, middleCall.SiteID}
			if classic {
				hir.Inline(p)
			} else if _, err := rewrite.Inline(p); err != nil {
				t.Fatal(err)
			}
			got := outer.Body.List[0].X
			if got.Kind != hir.New || got.SiteID != wantID || got.SiteSource != source || got.SiteOwner != owner || !reflect.DeepEqual(got.InlinePath, wantPath) {
				t.Fatalf("lost inline identity: %+v; want %v", got, wantPath)
			}
		})
	}
}
