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
