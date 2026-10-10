package grace

import (
	"reflect"
	"testing"
)

func TestRegionInvalidationNegation(t *testing.T) {
	src := `(rule denied 0 (head (denied ?m)) (base (bad ?m)))
 (rule allowed 0 (head (allowed ?m)) (base (method ?m) (not (denied ?m))))`
	_, rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}
	db := NewDB()
	db.regionFor = func(_ string, args Tuple) string { return args[0] }
	for _, m := range []string{"a", "b"} {
		if err := db.Add("method", m); err != nil {
			t.Fatal(err)
		}
	}
	if err := Evaluate(db, rules); err != nil {
		t.Fatal(err)
	}
	stable := db.tables["allowed"].rows[1]
	db.InvalidateRegions(map[string]bool{"a": true}, rules)
	if err := db.Add("method", "a"); err != nil {
		t.Fatal(err)
	}
	if err := db.Add("bad", "a"); err != nil {
		t.Fatal(err)
	}
	db.evaluateRegion = func(_ string, args Tuple) bool { return args[0] == "a" }
	if err := Evaluate(db, rules); err != nil {
		t.Fatal(err)
	}
	if db.Has("allowed", "a") || !db.Has("denied", "a") || !db.Has("allowed", "b") {
		t.Fatal("stale negative proof survived")
	}
	if db.tables["allowed"].rows[0] != stable {
		t.Fatal("stable proof was replaced")
	}
	affected := DependentRegions(map[string]bool{"a": true}, map[string][]string{"a": {"b"}, "b": {"c", "a"}})
	if !reflect.DeepEqual(affected, map[string]bool{"a": true, "b": true, "c": true}) {
		t.Fatal(affected)
	}
}
