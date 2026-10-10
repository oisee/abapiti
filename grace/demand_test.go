package grace

import (
	"reflect"
	"testing"
)

func TestDemandAndMatcher(t *testing.T) {
	db, rules, err := ParseRules(`
 (fact node s lit) (fact value s target)
 (fact method untouched)
 (rule deny 0 (head (denied ?s)) (base (bad ?s)))
 (rule allow 0 (head (allowed ?s ?v))
  (base (value ?s ?v) (not (denied ?s))))
 (rule unused 0 (head (unused ?m)) (base (method ?m)))
 (grace choose 0 (match (node ?s lit))
  (where (allowed ?s ?v)) (action (replace ?s ?v)))`)
	if err != nil {
		t.Fatal(err)
	}
	selected, needed := SelectDemand([]*Rules{rules}, rules.RewriteGoals())
	if !reflect.DeepEqual(needed, map[string]bool{"node": true, "allowed": true, "value": true, "denied": true, "bad": true}) {
		t.Fatal(needed)
	}
	db.SetDemand(needed)
	if db.Demands("unused") {
		t.Fatal("unrelated relation demanded")
	}
	if err := db.Add("unused", "ignored"); err != nil {
		t.Fatal(err)
	}
	if err := Evaluate(db, selected); err != nil {
		t.Fatal(err)
	}
	if db.Count("unused") != 0 {
		t.Fatal("unused relation evaluated")
	}
	rr := rules.Rewrites()[0]
	if err := rr.Validate(db); err != nil {
		t.Fatal(err)
	}
	matcher := rr.Matcher(db)
	if got := matcher.Match(Tuple{"s", "lit"}); !reflect.DeepEqual(got, []Tuple{{"s", "target"}}) {
		t.Fatal(got)
	}
	for _, node := range []Tuple{nil, {"s"}, {"s", "call"}, {"s", "lit", "extra"}} {
		if got := matcher.Match(node); len(got) != 0 {
			t.Fatal(got)
		}
	}
}

func TestLookupPreservesInsertionOrderAndCopies(t *testing.T) {
	db := NewDB()
	for _, tuple := range []Tuple{{"c", "m", "z"}, {"c", "m", "a"}, {"d", "m", "b"}} {
		if err := db.Add("dispatch", tuple...); err != nil {
			t.Fatal(err)
		}
	}
	got := db.Lookup("dispatch", "c", "m")
	if !reflect.DeepEqual(got, []Tuple{{"c", "m", "z"}, {"c", "m", "a"}}) {
		t.Fatal(got)
	}
	got[0][2] = "changed"
	if !db.Has("dispatch", "c", "m", "z") {
		t.Fatal("lookup exposed database storage")
	}
}
