package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestAssumeIntegerIncompleteHIR(t *testing.T) {
	// Failed expression lowering can leave nil runtime arguments/receivers.
	missingType := &hir.Expr{Kind: hir.Local}
	expressions := []*hir.Expr{
		{Kind: hir.RuntimeOp, Op: "string.slice", Type: hir.T(hir.String), X: &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.String), Value: "abc"}, Args: []*hir.Expr{nil, missingType}},
		{Kind: hir.RuntimeOp, Op: "number.toString", Type: hir.T(hir.String)},
		{Kind: hir.NumericConvert, Type: hir.T(hir.Number)},
		{Kind: hir.Binary, Op: "+", Type: hir.T(hir.Number)},
	}
	body := &hir.Stmt{Kind: hir.Block}
	for _, e := range expressions {
		body.List = append(body.List, &hir.Stmt{Kind: hir.ExprStmt, X: e})
	}
	l := &lowerer{out: &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Body: body}}}}}, floatSites: map[string]bool{}}
	l.assumeIntegerTypes()
	if expressions[0].Args[0] != nil || expressions[0].Args[1] != missingType || missingType.Type.Kind != "" {
		t.Fatal("missing expressions/types were invented")
	}
	if len(hir.Verify(l.out)) == 0 {
		t.Fatal("incomplete HIR must still be reported")
	}
}

func TestFailedDeclarationCascades(t *testing.T) {
	p := integerProbe(t, `export class Probe {
 run(s: string) { const failed = s.unsupported(); return failed; }
 other(failed: string) { return failed; }
 }`)
	_, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cascades := 0
	for _, d := range ds {
		if d.Cascade {
			cascades++
			if !strings.Contains(d.Message, "identifier failed ") {
				t.Fatal(d)
			}
		}
	}
	if cascades != 1 {
		t.Fatalf("got %d cascades: %v", cascades, ds)
	}
}
