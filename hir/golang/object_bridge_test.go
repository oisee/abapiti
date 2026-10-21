package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestObjectBridgeRejectsUnreviewedForeignReferences(t *testing.T) {
	dyn, root := hir.T(hir.Dynamic), hir.Ref(hir.RootObject)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "bridge", Static: true, Params: []hir.Param{{Name: "d", Type: dyn}}, Result: root, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.asObject", Type: root, X: hir.V("d", dyn)}})}}}}}
	main := `for _,v:=range []any{map[string]int{"x":1},[]int{1},func(){}} {func(){defer func(){if _,ok:=recover().(rangeFault);!ok {panic("unreviewed reference kind must refuse")}}();dynObject(box(v))}()};fmt.Println("refused")`
	if got := execute(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}
