package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestAuditCollectsEveryRefusal(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	number := hir.T(hir.Number)
	method := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(
		&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Node: hir.Node{ID: 1}, Kind: hir.RuntimeOp, Op: "string.parseFloat", Type: hir.T(hir.Optional, number), X: hir.L(hir.T(hir.String), "a")}},
		&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Node: hir.Node{ID: 2}, Kind: hir.RuntimeOp, Op: "string.parseIntNumber", Type: number, X: hir.L(hir.T(hir.String), "not a number")}},
	)}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Host", Methods: []*hir.Method{method}}}}
	if _, err := Emit(p); err == nil || !strings.Contains(err.Error(), "no emitter") {
		t.Fatalf("normal emitter must refuse: %v", err)
	}
	errors := Audit(p)
	if len(errors) != 2 {
		t.Fatalf("want both emitter refusals, got %v", errors)
	}
	for _, err := range errors {
		r, ok := err.(Refusal)
		if !ok || r.Method != method {
			t.Fatalf("missing owning body: %#v", err)
		}
	}
}
