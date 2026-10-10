package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
)

func TestValueSourceOrder(t *testing.T) {
	if sourceOrder("src/a.ts:9:20") >= sourceOrder("src/a.ts:10:1") {
		t.Fatal("lexical rather than numeric line order")
	}
	if stage("src/abap/2_statements/combi.ts.Sequence::run") != "statements/combi.Sequence" {
		t.Fatal("lost combi class")
	}
}

// The ordinary short suite exercises all synthetic rules; the explicit full
// gate also compares the independently evaluated rules over the entire closure.
func TestValueObjectsFullClosure(t *testing.T) {
	if testing.Short() || os.Getenv("ABAPITI_VALUE_OBJECTS_FULL") != "1" {
		t.Skip("set ABAPITI_VALUE_OBJECTS_FULL=1")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(home, ".cache/value-objects-reference")
	check := func(base *rewrite.DB) error {
		ref, steps, err := gracecheck.ReferenceFull(base, rewrite.ValueObjectRules(), 1000000000)
		if err != nil {
			return err
		}
		_, rules, err := rewrite.Parse(rewrite.ValueObjectRules())
		if err != nil {
			return err
		}
		if err = rewrite.Evaluate(base, rules); err != nil {
			return err
		}
		gracecheck.Equal(t, base, ref)
		t.Logf("full closure independent evaluator agrees; %d premises", steps)
		return nil
	}
	if err = runWithOptions(out, "", true, check); err != nil {
		t.Fatal(err)
	}
}
