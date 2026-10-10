// Package parallel proves conditional parallel maps from reviewed effect facts.
// It neither rewrites HIR nor starts runtime workers. It is deliberately absent
// from the default inliner's dependency graph.
package parallel

import (
	"embed"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir/rewrite"
)

//go:embed rules.grace
var sources embed.FS

// Source returns the proof rules for independent reference evaluation.
func Source() string {
	b, err := sources.ReadFile("rules.grace")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Analyze copies the evidence and evaluates proof obligations. The caller must
// supply complete effect/call summaries and ownership certificates as described
// in README.md. Missing summaries and unknown targets block a proof.
func Analyze(evidence *rewrite.DB) (*rewrite.DB, error) {
	if evidence == nil {
		return nil, fmt.Errorf("nil parallel evidence")
	}
	// Derived proofs must never become trusted inputs on a later invocation.
	// In particular, adding an unknown effect must not preserve an old mark.
	for _, pred := range strings.Fields(`mark_parallel p_edge p_resolved p_reach
  p_block p_blocked p_memo_write p_counter_write p_allowed_write p_written
  p_not_memo p_allowed_read p_initialized p_throwing p_visible_loop`) {
		if evidence.Count(pred) != 0 {
			return nil, fmt.Errorf("parallel evidence contains derived predicate %s; supply fresh base facts", pred)
		}
	}
	db := rewrite.NewDB()
	for _, pred := range evidence.Predicates() {
		for _, row := range evidence.Facts(pred) {
			if err := db.Add(pred, row...); err != nil {
				return nil, err
			}
		}
	}
	_, rules, err := rewrite.Parse(Source())
	if err != nil {
		return nil, err
	}
	if err := rewrite.Evaluate(db, rules); err != nil {
		return nil, err
	}
	return db, nil
}
