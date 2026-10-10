package rewrite_test

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
	"testing"
)

func TestStoreReferenceEvaluator(t *testing.T) {
	i := hir.T(hir.I32)
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: i, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "dead", Type: i, X: hir.L(i, 7)},
		&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, 3)},
		&hir.Stmt{Kind: hir.Return, X: hir.V("v", i)},
	)}}}}}
	db, err := rewrite.ExtractRewriteFacts(p)
	if err != nil {
		t.Fatal(err)
	}
	// Independently scanned joins must select the same copy and dead store.
	source := gracecheck.Source(t, "stores") + `
 (rule copy-selection 0 (head (selected_copy ?use ?def))
  (base (copy_proven ?use ?def ?v) (def ?v ?def) (use ?v ?use) (local_ref ?use ?v)))
 (rule dead-selection 0 (head (selected_dead ?site))
  (base (store_inert ?site ?v) (def ?v ?site) (not_read_after ?v ?site)))`
	got := gracecheck.Evaluate(t, db, source)
	oracle, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, store := range db.Facts("store_inert") {
		if got.Has("not_read_after", store[1], store[0]) == oracle.Has("live_out", store[1], store[0]) {
			t.Fatalf("contracted liveness differs at %v", store)
		}
	}
	if got.Count("selected_copy") != 1 || got.Count("selected_dead") != 1 {
		t.Fatal("unexpected rule selection")
	}
}
