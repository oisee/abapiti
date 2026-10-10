package rewrite

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestEnsureInitMatchesEmitterGuards(t *testing.T) {
	i := hir.T(hir.I32)
	initA := method("class_constructor", store(get("A", "constant", i), hir.L(i, 7)), store(get("A", "value", i), hir.L(i, 0)))
	bump := method("bump", store(get("A", "value", i), hir.L(i, 1))) // disqualifies value
	entry := method("entry")
	ctor := &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}
	a := &hir.Class{Name: "A", Fields: []hir.Field{{Name: "constant", Static: true, Type: i}, {Name: "value", Static: true, Type: i}}, Methods: []*hir.Method{initA, bump, entry}, Ctor: ctor}
	b := &hir.Class{Name: "B", Fields: []hir.Field{{Name: "value", Static: true, Type: i}}, Methods: []*hir.Method{method("class_constructor", store(get("B", "value", i), hir.L(i, 42))), method("entry")}}
	use := method("use", exp(get("A", "constant", i)), exp(get("A", "value", i)), store(get("A", "value", i), hir.L(i, 2)), exp(&hir.Expr{Kind: hir.New, Type: hir.Ref("A")}), exp(get("B", "value", i)), exp(&hir.Expr{Kind: hir.New, Type: hir.Ref("B")}))
	d := analyze(t, a, b, &hir.Class{Name: "User", Methods: []*hir.Method{use}})
	for _, row := range []struct{ m, s, c string }{
		{"A::entry", "A::entry/entry", "A"}, {"A::constructor", "A::constructor/entry", "A"},
		{"User::use", "User::use/body/s1/x", "A"}, {"User::use", "User::use/body/s2", "A"}, {"User::use", "User::use/body/s3/x", "A"},
	} {
		assertFact(t, d, true, "ensure_init", row.m, row.s, row.c)
	}
	assertFact(t, d, false, "ensure_init", "User::use", "User::use/body/s0/x", "A")
	for _, r := range d.Facts("ensure_init") {
		if r[2] == "B" {
			t.Errorf("constant-only B must not initialize: %v", r)
		}
	}
	assertFact(t, d, true, "writes_static_transitive", "A::entry", "A", "value")
	assertFact(t, d, false, "writes_static_transitive", "B::entry", "A", "value")
}

func TestFieldFactsKeepOwnerAndNameSeparate(t *testing.T) {
	i := hir.T(hir.I32)
	// These keys collide if encoded as owner+"."+name.
	a := &hir.Class{Name: "A.B", Fields: []hir.Field{{Name: "C", Static: true, Type: i}}, Methods: []*hir.Method{method("class_constructor", store(get("A.B", "C", i), hir.L(i, 1)))}}
	b := &hir.Class{Name: "A", Fields: []hir.Field{{Name: "B.C", Static: true, Type: i}}, Methods: []*hir.Method{method("class_constructor", store(get("A", "B.C", i), hir.L(i, 2))), method("write", store(get("A", "B.C", i), hir.L(i, 3)))}}
	reader := &hir.Class{Name: "R", Methods: []*hir.Method{method("read", exp(get("A.B", "C", i)), exp(get("A", "B.C", i)))}}
	d := analyze(t, a, b, reader)
	assertFact(t, d, true, "reads_static", "R::read", "A.B", "C")
	assertFact(t, d, true, "reads_static", "R::read", "A", "B.C")
	assertFact(t, d, false, "ensure_init", "R::read", "R::read/body/s0/x", "A.B")
	assertFact(t, d, true, "ensure_init", "R::read", "R::read/body/s1/x", "A")
	// An instance shadow does not change the owner of an inherited static.
	base := &hir.Class{Name: "Base", Fields: []hir.Field{{Name: "n", Static: true, Type: i}}}
	sub := &hir.Class{Name: "Sub", Super: "Base", Fields: []hir.Field{{Name: "n", Type: i}}, Methods: []*hir.Method{method("read", exp(get("Sub", "n", i)))}}
	facts := Extract(&hir.Program{Classes: []*hir.Class{base, sub}})
	assertFact(t, facts, true, "reads_static", "Sub::read", "Base", "n")
}
