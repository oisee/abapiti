package rewrite

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestReplaceRoundSnapshot(t *testing.T) {
	num := hir.T(hir.Number)
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: num, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: hir.L(num, 1)}, &hir.Stmt{Kind: hir.Return, X: hir.L(num, 2)})}}}}}
	_, rules, err := Parse(`
 (grace swap 10
  (match (node ?s lit))
  (where (node ?t lit) (not (node ?t virtual)))
  (action (replace ?s ?t)) (bound depth 1))`)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := Rewrite(p, rules, Limits{Rounds: 2})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rounds != 2 {
		t.Fatal(stats)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
}
func TestRewriteParseRejectsUnprovedActions(t *testing.T) {
	for _, src := range []string{
		`(grace bad 0 (match (node ?s virtual)) (where) (action (inline ?other)))`,
		`(grace bad 0 (match (node ?s virtual)) (where) (action (unknown ?s)))`,
		`(grace bad 0 (match (node ?s virtual)) (where (not (pure ?m))) (action (inline ?s)))`,
		`(grace bad 0 (match (node ?s virtual)) (where) (action (inline ?s)) (bound depth -1))`,
	} {
		if _, _, err := Parse(src); err == nil {
			t.Fatalf("accepted %s", src)
		}
	}
}
