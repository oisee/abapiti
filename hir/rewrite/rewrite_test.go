package rewrite

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestReplaceRoundSnapshot(t *testing.T) {
	num := hir.T(hir.Number)
	fixture := func() *hir.Program {
		return &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Params: []hir.Param{{Name: "x", Type: num}, {Name: "y", Type: num}}, Result: num, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: hir.L(num, 1)}, &hir.Stmt{Kind: hir.ExprStmt, X: hir.V("x", num)}, &hir.Stmt{Kind: hir.Return, X: hir.V("y", num)})}}}}}
	}
	_, rules, err := Parse(`
 (grace fallback 0 (match (node "C::run/body/s0/x" lit)) (where)
  (action (replace "C::run/body/s0/x" "C::run/body/s2/x")))
 (grace first 10 (match (node "C::run/body/s0/x" lit)) (where)
  (action (replace "C::run/body/s0/x" "C::run/body/s1/x")))
 (grace tied 10 (match (node "C::run/body/s0/x" lit)) (where)
  (action (replace "C::run/body/s0/x" "C::run/body/s2/x")))
 (grace next-round 20 (match (node "C::run/body/s0/x" local)) (where)
  (action (replace "C::run/body/s0/x" "C::run/body/s2/x")))`)
	if err != nil {
		t.Fatal(err)
	}
	for rounds, want := range map[int]string{1: "x", 2: "y"} {
		p := fixture()
		stats, err := Rewrite(p, rules, Limits{Rounds: rounds})
		if err != nil {
			t.Fatal(err)
		}
		got := p.Classes[0].Methods[0].Body.List[0].X.Name
		if got != want || stats.Rounds != rounds {
			t.Fatalf("rounds=%d: got %s %+v, want %s", rounds, got, stats, want)
		}
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
