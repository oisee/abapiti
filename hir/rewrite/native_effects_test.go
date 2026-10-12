package rewrite

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestNativeDispatchCoercionAndLexicalIdentityEffects(t *testing.T) {
	for _, op := range []string{"dynamic.call0", "dynamic.toString"} {
		e := RuntimeEffects[op]
		if !e.Reads.Global || e.Writes != "Global" || !e.Allocates || e.MayRaise != "Catchable" || e.Conservative == "" {
			t.Fatal(op, e)
		}
	}
	fresh := &hir.Expr{Kind: hir.FreshClassOf, Owner: "Local", Type: hir.T(hir.ClassValue)}
	m := method("make", ret(fresh))
	m.Result = hir.T(hir.ClassValue)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Local", Scoped: true}, {Name: "Probe", Methods: []*hir.Method{m}}}}
	db := Extract(p)
	found := false
	for _, row := range db.Facts("unknown_effect") {
		found = found || row[0] == "Probe::make"
	}
	if !found {
		t.Fatal("fresh lexical identity was classified as pure")
	}
}
