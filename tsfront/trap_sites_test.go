package tsfront

import (
	"reflect"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestTrapSitesClassifiesBodyGuardAndStatementTraps(t *testing.T) {
	trap := func(source, name string) *hir.Stmt {
		return &hir.Stmt{Node: hir.Node{Source: source}, Kind: hir.Trap, Name: name}
	}
	boolean := hir.T(hir.Bool)
	guarded := &hir.Stmt{Kind: hir.If, X: hir.L(boolean, true), Body: hir.B(trap("a.ts:3:5", "a.ts:3:5: guard"))}
	inSeq := &hir.Expr{Kind: hir.Seq, Type: boolean, Stmt: hir.B(trap("a.ts:4:1", "a.ts:4:1")), Y: hir.L(boolean, true)}
	p := &hir.Program{Classes: []*hir.Class{{
		Name: "a.ts.module",
		Methods: []*hir.Method{
			{Node: hir.Node{Source: "a.ts:1:1"}, Name: "refused", Body: hir.B(trap("", "lowering trap: a.ts:1:1: no"))},
			{Node: hir.Node{Source: "a.ts:2:1"}, Name: "mixed", Body: hir.B(guarded, &hir.Stmt{Kind: hir.Return, X: inSeq}, trap("a.ts:5:1", "a.ts:5:1"))},
		},
	}}}
	want := []TrapSite{
		{"a.ts.module", "refused", "a.ts:1:1", "a.ts:1:1", "body", "lowering trap: a.ts:1:1: no"},
		{"a.ts.module", "mixed", "a.ts:2:1", "a.ts:3:5", "guard", "a.ts:3:5: guard"},
		{"a.ts.module", "mixed", "a.ts:2:1", "a.ts:4:1", "stmt", "a.ts:4:1"},
		{"a.ts.module", "mixed", "a.ts:2:1", "a.ts:5:1", "stmt", "a.ts:5:1"},
	}
	if got := TrapSites(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
