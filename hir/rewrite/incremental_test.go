package rewrite

import (
	"reflect"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func incrementalFixture() *hir.Program {
	void := hir.T(hir.Void)
	call := func(name string) *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.VirtualCall, Name: name, Type: void, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}}
	}
	var body []*hir.Stmt
	for i := 0; i < 13; i++ {
		body = append(body, call("empty"))
	}
	return &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{
		{Name: "caller", Virtual: true, Result: void, Body: hir.B(call("large"))},
		{Name: "large", Virtual: true, Result: void, Body: hir.B(body...)},
		{Name: "empty", Virtual: true, Result: void, Body: hir.B()},
		{Name: "unrelated", Virtual: true, Result: void, Body: hir.B()},
	}}}}
}

func TestIncrementalRoundFacts(t *testing.T) {
	b, err := ruleFiles.ReadFile("rules/inline.grace")
	if err != nil {
		t.Fatal(err)
	}
	_, rules, err := Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	run := func(mode string) (Stats, string, []map[string][]Tuple) {
		t.Setenv("ABAPITI_GRACE_RECOMPUTE", mode)
		p := incrementalFixture()
		var snapshots []map[string][]Tuple
		var stable *row
		stats, err := rewriteObserved(p, rules, Limits{Rounds: 4}, func(db *DB) {
			snapshot := map[string][]Tuple{}
			for _, pred := range db.Predicates() {
				if facts := db.Facts(pred); len(facts) > 0 {
					snapshot[pred] = facts
				}
			}
			snapshots = append(snapshots, snapshot)
			if mode != "full" {
				for _, row := range db.tables["inline_candidate"].rows {
					if row.args[0] == "C::unrelated" {
						if stable != nil && stable != row {
							t.Fatal("unrelated method proof was recomputed")
						}
						stable = row
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		return stats, hir.Dump(p), snapshots
	}
	a, adump, afacts := run("")
	bstats, bdump, bfacts := run("full")
	if !reflect.DeepEqual(a, bstats) || adump != bdump {
		t.Fatalf("incremental/full outputs differ: %+v / %+v", a, bstats)
	}
	if !reflect.DeepEqual(afacts, bfacts) {
		t.Fatal("incremental/full round relations differ")
	}
	if a.CallSites != 14 || a.Rounds != 3 {
		t.Fatalf("fixture must rewrite in two rounds and terminate: %+v", a)
	}
}

func TestRegionInvalidationNegation(t *testing.T) {
	src := `(rule denied 0 (head (denied ?m)) (base (bad ?m)))
 (rule allowed 0 (head (allowed ?m)) (base (method ?m) (not (denied ?m))))`
	_, rules, err := Parse(src)
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
	db.invalidateRegions(map[string]bool{"a": true}, rules)
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
	affected := dependentRegions(map[string]bool{"a": true}, map[string][]string{"a": {"b"}, "b": {"c", "a"}})
	if !reflect.DeepEqual(affected, map[string]bool{"a": true, "b": true, "c": true}) {
		t.Fatal(affected)
	}
}
