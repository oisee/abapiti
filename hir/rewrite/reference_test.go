package rewrite_test

import (
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
	"testing"
)

func TestReferenceEvaluator(t *testing.T) {
	src := `(fact edge a b) (fact edge b c) (fact edge c a)
 (rule reach 0 (head (reach ?a ?b)) (base (edge ?a ?b)) (tail (reach ?a ?x) (edge ?x ?b)))
 (rule bounded 0 (head (near ?a ?b)) (bound depth 2) (base (edge ?a ?b)) (tail (near ?a ?x) (edge ?x ?b)))`
	base, _, e := rewrite.Parse(src)
	if e != nil {
		t.Fatal(e)
	}
	gracecheck.Evaluate(t, base, src)
}

func TestIndexedBindingAndWideTuples(t *testing.T) {
	src := `(fact pairs a a) (fact pairs a b) (fact pairs b b)
 (fact wide a b c d e f g h i j) (fact wide a b c d e f g h i k)
 (rule diagonal 0 (head (diagonal ?x)) (base (pairs ?x ?x)))
 (rule wide 0 (head (last ?i ?j)) (base (wide a b c d e f g h ?i ?j)))
 (rule negative 0 (head (missing ?x)) (base (diagonal ?x) (not (pairs ?x c))))`
	base, _, err := rewrite.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	gracecheck.Evaluate(t, base, src)
}
