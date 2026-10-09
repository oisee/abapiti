package rewrite

import (
	"reflect"
	"testing"
)

func TestFixedPoint(t *testing.T) {
	src := `(fact edge a b) (fact edge b c) (fact edge c a)
 (rule reach 1 (head (reach ?a ?b)) (base (edge ?a ?b))
 (tail (reach ?a ?x) (edge ?x ?b)))
 (rule bounded 0 (head (near ?a ?b)) (bound depth 2)
 (base (edge ?a ?b)) (tail (near ?a ?x) (edge ?x ?b)))
 (rule missing 0 (head (missing ?a ?b)) (base (edge ?a ?b) (not (edge ?b ?a))))`
	d, r, e := Parse(src)
	if e != nil {
		t.Fatal(e)
	}
	if e = Evaluate(d, r); e != nil {
		t.Fatal(e)
	}
	if d.Count("reach") != 9 || d.Count("near") != 6 || d.Count("missing") != 3 {
		t.Fatalf("counts %d %d %d", d.Count("reach"), d.Count("near"), d.Count("missing"))
	}
	before := d.Facts("reach")
	if e = Evaluate(d, r); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, d.Facts("reach")) {
		t.Fatal("unstable fixed point")
	}
}
func TestRejectInvalid(t *testing.T) {
	for _, src := range []string{`(`, `(fact x ?a)`, `(rule x 0 (head (x ?a)) (base (y ?b)))`, `(rule x 0 (head (x a)) (base (not (x a))))`, `(fact x a) (fact x a b)`, `(rule x 0 (head (x _)) (base (y a)))`, `(rule x 0 (head (x a)) (bound depth -1) (base))`} {
		d, r, e := Parse(src)
		if e == nil {
			e = Evaluate(d, r)
		}
		if e == nil {
			t.Errorf("accepted %s", src)
		}
	}
}
func TestQuotedConstants(t *testing.T) {
	d, r, e := Parse(`; comment
(fact name "?literal" "a b\\c") (rule x 0 (head (ok ?x)) (base (name "?literal" ?x)))`)
	if e != nil {
		t.Fatal(e)
	}
	if e = Evaluate(d, r); e != nil {
		t.Fatal(e)
	}
	if !d.Has("ok", `a b\c`) {
		t.Fatal(d.Facts("ok"))
	}
}
