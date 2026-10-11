package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestStringRepeatJSCounts(t *testing.T) {
	main := `
 check:=func(ok bool){if !ok{panic("repeat semantics")}}
 fault:=func(f func()){defer func(){check(recover()!=nil)}();f()}
 var zero float64; nan:=zero/zero; inf:=1/zero
 check(str("ab").repeat(2.9)==str("abab"))
 check(str("a").repeat(-0.9)==str(""))
 check(str("a").repeat(nan)==str(""))
 check(str("").repeat(1e30)==str(""))
 check(unit(0xd800).repeat(2)==unit(0xd800)+unit(0xd800))
 fault(func(){str("a").repeat(-1)})
 fault(func(){str("a").repeat(inf)})
 fault(func(){str("").repeat(-inf)})
 fault(func(){str("ab").repeat(2147483647)})
 fmt.Println("ok")`
	p := &hir.Program{Classes: []*hir.Class{{Name: "Repeat", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.repeat", Type: hir.T(hir.String), X: hir.L(hir.T(hir.String), "a"), Args: []*hir.Expr{hir.L(hir.T(hir.Number), 2)}}})}}}}}
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
