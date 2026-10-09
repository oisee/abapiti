package hirclone

import (
	"reflect"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestClonePreservesNilTypesAndSharing(t *testing.T) {
	e := hir.L(hir.T(hir.Array, hir.T(hir.I64)), int64(9007199254740993))
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "m", Body: hir.B(nil, &hir.Stmt{Kind: hir.ExprStmt, X: e}, &hir.Stmt{Kind: hir.ExprStmt, X: e})}}}}}
	q := Clone(p)
	if !reflect.DeepEqual(p, q) {
		t.Fatal("copy changed HIR or literal types")
	}
	list := q.Classes[0].Methods[0].Body.List
	if list[0] != nil || list[1].X != list[2].X || list[1].X == e {
		t.Fatal("nil entries or internal pointer sharing changed")
	}
	list[1].X.Value = int64(2)
	list[1].X.Type.Args[0].Kind = hir.String
	if e.Value != int64(9007199254740993) || e.Type.Args[0].Kind != hir.I64 {
		t.Fatal("copy shares mutable data with input")
	}
	if Clone(nil) != nil {
		t.Fatal("nil program changed")
	}
}
