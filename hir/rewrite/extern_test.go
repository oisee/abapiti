package rewrite

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestExternEffects(t *testing.T) {
	call := &hir.Expr{Kind: hir.ExternCall, Owner: "example.org/host", Name: "Read", Type: hir.T(hir.String), Extern: &hir.ExternSignature{Params: []hir.Type{hir.T(hir.String)}}, Args: []*hir.Expr{hir.L(hir.T(hir.String), "file")}}
	m := method("run", ret(call))
	m.Result = hir.T(hir.String)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}}
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	db := Extract(p)
	for _, pred := range []string{"unknown_effect", "raises", "sink", "op_allocates", "op_may_raise", "op_reads"} {
		if len(db.Facts(pred)) == 0 {
			t.Errorf("missing %s", pred)
		}
	}
}
