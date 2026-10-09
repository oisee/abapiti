package gracecheck

import (
	"github.com/oisee/abapiti/hir/rewrite"
	"testing"
)

func TestReferenceSemantics(t *testing.T) {
	for _, src := range []string{
		`(fact edge a b) (fact edge b c) (fact edge c a)
 (rule reach 1 (head (reach ?a ?b)) (base (edge ?a ?b)) (tail (reach ?a ?x) (edge ?x ?b)))
 (rule near 0 (head (near ?a ?b)) (bound depth 2) (base (edge ?a ?b)) (tail (near ?a ?x) (edge ?x ?b)))
 (rule missing 0 (head (missing ?a ?b)) (base (not (edge ?b ?a)) (edge ?a ?b)))`,
		`(fact ready) (rule long 10 (head (middle a)) (base (ready)))
 (rule longer 9 (head (seed a)) (base (middle a))) (rule short 0 (head (seed a)) (base (ready)))
 (rule bound 0 (head (answer)) (base (seed a)) (bound depth 2))`,
		`(fact size small 3) (fact size big 13) (fact size unknown "?")
 (rule large 0 (head (large ?m)) (base (size ?m ?n) (not (le ?n 12))))`,
	} {
		base, _, e := rewrite.Parse(src)
		if e != nil {
			t.Fatal(e)
		}
		want := Evaluate(t, base, src)
		got, _, err := ReferenceFull(base, src, 1000000000)
		if err != nil {
			t.Fatal(err)
		}
		Equal(t, got, want)
	}
	base, _, _ := rewrite.Parse(`(fact x a)`)
	if _, _, e := Reference(base, `(rule x 0 (head (y ?a)) (base (x ?a)))`, 0); e == nil {
		t.Fatal("step cap ignored")
	}
}
