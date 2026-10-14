package tsfront

import (
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A failed lowering can leave operands nil; the range pass must not panic on
// it, so the full diagnostic inventory still reaches the caller.
func TestNumberRangesToleratePartialHIR(t *testing.T) {
	num := hir.T(hir.Number)
	x := &hir.Expr{Kind: hir.Local, Name: "x", Type: num}
	for name, e := range map[string]*hir.Expr{
		"binary":    {Kind: hir.Binary, Op: "+", X: x, Type: num},
		"unary":     {Kind: hir.Unary, Op: "-", Type: num},
		"minmax":    {Kind: hir.NumericMinMax, Type: num},
		"seq":       {Kind: hir.Seq, Type: num},
		"binary-yx": {Kind: hir.Binary, Op: "+", Y: x, Type: num},
	} {
		p := &numberPass{}
		info := &numberMethod{exprs: map[*hir.Expr]numberInterval{}, locals: map[*hir.Stmt]numberInterval{}}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panic on partial HIR: %v", name, r)
				}
			}()
			p.rewriteExpr(e, map[string]hir.Type{}, info)
		}()
	}
}
