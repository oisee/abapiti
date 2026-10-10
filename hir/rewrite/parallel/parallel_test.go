package parallel_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/hir/rewrite/parallel"
	"github.com/oisee/abapiti/internal/gracecheck"
)

const isolated = `(fact p_loop L entry) (fact p_method entry)
 (fact p_complete entry) (fact p_isolated L) (fact p_slot_plan L)
 (fact p_owned L local)`

func TestParallelObligations(t *testing.T) {
	tests := []struct {
		name, facts string
		yes         bool
		condition   string
	}{
		{"fresh receiver graph", `(fact p_write entry local store)`, true, "source-order-slots"},
		{"shared receiver", `(fact p_write entry shared store)`, false, ""},
		{"memo", memoFacts("key", "value", "invalidation"), true, "coherent-memo"},
		{"memo impure compute", strings.ReplaceAll(memoFacts("key", "value", "invalidation"), `(fact p_pure compute)`, ""), false, ""},
		{"memo missing key dependency", memoFacts("", "value", "invalidation"), false, ""},
		{"memo observable identity", memoFacts("key", "", "invalidation"), false, ""},
		{"memo invalidation", memoFacts("key", "value", ""), false, ""},
		{"memo unrelated reader", memoFacts("key", "value", "invalidation") + `(fact p_call entry reader S) (fact p_method reader) (fact p_complete reader) (fact p_read reader cache)`, false, ""},
		{"exact unobserved counter", `(fact p_write entry count counter) (fact p_exact_counter entry count) (fact p_counter_unobserved L count)`, true, "atomic-counter"},
		{"floating point counter", `(fact p_write entry count counter) (fact p_counter_unobserved L count)`, false, ""},
		{"observed counter", `(fact p_write entry count counter) (fact p_exact_counter entry count) (fact p_counter_unobserved L count) (fact p_read entry count)`, false, ""},
		{"warm before loop", `(fact p_init entry C) (fact p_warmed L C)`, true, "source-order-slots"},
		{"warm precondition", `(fact p_init entry C) (fact p_warm_plan L C)`, true, "warm"},
		{"nested cold lazy init", `(fact p_init entry C) (fact p_warmed L C) (fact p_call entry nested S) (fact p_method nested) (fact p_complete nested) (fact p_init nested D)`, false, ""},
		{"cold lazy init", `(fact p_init entry C)`, false, ""},
		{"ordered appends", `(fact p_write entry result append) (fact p_ordered_plan L result)`, true, "ordered-join"},
		{"unordered appends", `(fact p_write entry result append)`, false, ""},
		{"append length observed", `(fact p_write entry result append) (fact p_ordered_plan L result) (fact p_read entry result)`, false, ""},
		{"shared readonly", `(fact p_read entry shared)`, true, "source-order-slots"},
		{"transitive shared dependency", `(fact p_read entry shared) (fact p_call entry writer S) (fact p_method writer) (fact p_complete writer) (fact p_write writer shared store)`, false, ""},
		{"throwing isolated", `(fact p_throw entry) (fact p_exception_plan L)`, true, "lowest-index-exception"},
		{"nonthrowing visible effect", `(fact p_visible entry)`, false, ""},
		{"throwing visible", `(fact p_throw entry) (fact p_exception_plan L) (fact p_visible entry)`, false, ""},
		{"throwing counter", `(fact p_throw entry) (fact p_exception_plan L) (fact p_write entry count counter) (fact p_exact_counter entry count) (fact p_counter_unobserved L count)`, false, ""},
		{"missing exception join", `(fact p_throw entry)`, false, ""},
		{"unknown runtime", `(fact p_unknown entry)`, false, ""},
		{"unknown direct target", `(fact p_call entry absent S)`, false, ""},
		{"empty virtual targets", `(fact p_virtual entry S) (fact p_dispatch_complete S)`, false, ""},
		{"incomplete virtual receivers", `(fact p_virtual entry S) (fact p_target S safe) (fact p_method safe) (fact p_complete safe)`, false, ""},
		{"virtual fresh receiver", virtualFacts + `(fact p_write safe local store)`, true, "source-order-slots"},
		{"virtual unsafe receiver", virtualFacts + `(fact p_target S unsafe) (fact p_method unsafe) (fact p_complete unsafe) (fact p_write unsafe shared store)`, false, ""},
		{"recursive pure graph", `(fact p_call entry rec S) (fact p_method rec) (fact p_complete rec) (fact p_call rec entry T)`, true, "source-order-slots"},
		{"recursive unknown graph", `(fact p_call entry rec S) (fact p_method rec) (fact p_complete rec) (fact p_call rec entry T) (fact p_unknown rec)`, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, _, err := rewrite.Parse(isolated + tt.facts)
			if err != nil {
				t.Fatal(err)
			}
			before := snapshot(base)
			got, err := parallel.Analyze(base)
			if err != nil {
				t.Fatal(err)
			}
			if yes := got.Count("mark_parallel") == 1; yes != tt.yes {
				t.Fatalf("proven=%v, want %v; blockers=%v", yes, tt.yes, got.Facts("p_block"))
			}
			if !reflect.DeepEqual(before, snapshot(base)) {
				t.Fatal("input mutated")
			}
			ref := gracecheck.Evaluate(t, base, parallel.Source())
			gracecheck.Equal(t, got, ref)
			if tt.condition != "" {
				found := false
				for _, row := range got.Facts("parallel_precondition") {
					if row[1] == tt.condition {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing condition %s", tt.condition)
				}
			}
			reversed := rewrite.NewDB()
			preds := base.Predicates()
			for i := len(preds) - 1; i >= 0; i-- {
				rows := base.Facts(preds[i])
				for j := len(rows) - 1; j >= 0; j-- {
					if err := reversed.Add(preds[i], rows[j]...); err != nil {
						t.Fatal(err)
					}
				}
			}
			again, err := parallel.Analyze(reversed)
			if err != nil {
				t.Fatal(err)
			}
			gracecheck.Equal(t, got, again)
		})
	}
}

const virtualFacts = `(fact p_virtual entry S) (fact p_dispatch_complete S)
 (fact p_target S safe) (fact p_method safe) (fact p_complete safe)`

func memoFacts(key, value, invalidation string) string {
	s := `(fact p_write entry cache memo) (fact p_read entry cache)
 (fact p_memo_read entry cache) (fact p_memo entry cache compute)
 (fact p_pure compute) (fact p_call entry compute compute-site)
 (fact p_method compute) (fact p_complete compute)`
	if key != "" {
		s += `(fact p_key_complete entry cache)`
	}
	if value != "" {
		s += `(fact p_value_semantic entry cache)`
	}
	if invalidation != "" {
		s += `(fact p_no_invalidation L cache)`
	}
	return s
}
func snapshot(db *rewrite.DB) map[string][]rewrite.Tuple {
	out := map[string][]rewrite.Tuple{}
	for _, p := range db.Predicates() {
		out[p] = db.Facts(p)
	}
	return out
}

func TestIncompleteAndUnisolated(t *testing.T) {
	for _, source := range []string{
		`(fact p_loop L entry) (fact p_method entry) (fact p_isolated L) (fact p_slot_plan L)`,
		`(fact p_loop L entry) (fact p_method entry) (fact p_complete entry) (fact p_slot_plan L)`,
		`(fact p_loop L entry) (fact p_method entry) (fact p_complete entry) (fact p_isolated L)`,
	} {
		base, _, err := rewrite.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		db, err := parallel.Analyze(base)
		if err != nil {
			t.Fatal(err)
		}
		if db.Count("mark_parallel") != 0 {
			t.Fatal("missing contract accepted")
		}
		gracecheck.Evaluate(t, base, parallel.Source())
	}
	if _, err := parallel.Analyze(nil); err == nil {
		t.Fatal("nil evidence accepted")
	}
}

func TestRejectStaleProofInputs(t *testing.T) {
	base, _, err := rewrite.Parse(isolated)
	if err != nil {
		t.Fatal(err)
	}
	proven, err := parallel.Analyze(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := proven.Add("p_unknown", "entry"); err != nil {
		t.Fatal(err)
	}
	if _, err := parallel.Analyze(proven); err == nil {
		t.Fatal("stale derived proof accepted as base evidence")
	}
}
