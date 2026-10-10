package main

import (
	"testing"

	"github.com/oisee/abapiti/hir/rewrite/parallel"
	"github.com/oisee/abapiti/internal/gracecheck"
)

func TestPinnedCandidateProof(t *testing.T) {
	base, err := evidence()
	if err != nil {
		t.Fatal(err)
	}
	db, err := parallel.Analyze(base)
	if err != nil {
		t.Fatal(err)
	}
	if rows := db.Facts("mark_parallel"); len(rows) != 1 || rows[0][0] != "lexer-files" {
		t.Fatalf("unexpected proofs: %v", rows)
	}
	gracecheck.Evaluate(t, base, parallel.Source())
	old, err := baseline(base)
	if err != nil {
		t.Fatal(err)
	}
	before, err := parallel.Analyze(old)
	if err != nil {
		t.Fatal(err)
	}
	if before.Count("mark_parallel") != 0 {
		t.Fatal("incomplete graph proved")
	}
	gracecheck.Evaluate(t, old, parallel.Source())
	a, err := report()
	if err != nil {
		t.Fatal(err)
	}
	b, err := report()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("nondeterministic report")
	}
}
