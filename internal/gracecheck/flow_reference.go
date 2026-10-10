package gracecheck

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir/rewrite"
)

const localFlowRules = `
(rule read 0 (head (live_in ?v ?s)) (base (use ?v ?s)))
(rule successor 0 (head (live_out ?v ?s)) (base (next ?s ?n) (live_in ?v ?n)))
(rule through 0 (head (live_in ?v ?s)) (base (live_out ?v ?s) (not (def ?v ?s))))
(rule last 0 (head (not_read_after ?v ?s)) (base (observed ?v ?s) (not (live_out ?v ?s))))
`

// Method-local CFG components are disjoint. Evaluate their unchanged rules with
// the independent snapshot reference before the global rules, so the longest
// method does not force thousands of unrelated methods through every round.
// No facts from the production engine are used. Cartesian Reference remains
// unfactored and checks this optimization on all ordinary fixtures.
func methodFlowReference(base *rewrite.DB, source string, cap int) (*rewrite.DB, int, error) {
	cs, err := parse(source)
	if err != nil {
		return nil, 0, err
	}
	output := map[string]bool{"live_in": true, "live_out": true, "not_read_after": true}
	input := map[string]bool{"def": true, "use": true, "observed": true, "next": true}
	var local, global []clause
	for _, c := range cs {
		if input[c.head.pred] {
			return reference(base, source, cap, true)
		}
		if output[c.head.pred] {
			local = append(local, c)
		} else {
			for _, a := range c.body {
				if output[a.pred] {
					// Preserve proof-depth semantics for arbitrary consumers.
					return reference(base, source, cap, true)
				}
			}
			global = append(global, c)
		}
	}
	expected, _ := parse(localFlowRules)
	if !reflect.DeepEqual(local, expected) {
		return reference(base, source, cap, true)
	}
	owners := map[string]string{}
	for _, r := range base.Facts("flow_node") {
		if len(r) != 2 || (owners[r[1]] != "" && owners[r[1]] != r[0]) {
			return reference(base, source, cap, true)
		}
		owners[r[1]] = r[0]
	}
	parts := map[string]*rewrite.DB{}
	locals := map[string]string{}
	for _, pred := range []string{"def", "use", "observed", "next", "live_in", "live_out", "not_read_after"} {
		for _, r := range base.Facts(pred) {
			if len(r) != 2 {
				return reference(base, source, cap, true)
			}
			method := owners[r[1]]
			if method == "" || (pred == "next" && owners[r[0]] != method) {
				return reference(base, source, cap, true)
			}
			if pred != "next" {
				if locals[r[0]] != "" && locals[r[0]] != method {
					return reference(base, source, cap, true)
				}
				locals[r[0]] = method
			}
			if parts[method] == nil {
				parts[method] = rewrite.NewDB()
			}
			if err := parts[method].Add(pred, r...); err != nil {
				return nil, 0, err
			}
		}
	}
	seed := rewrite.NewDB()
	for _, pred := range base.Predicates() {
		for _, r := range base.Facts(pred) {
			if err := seed.Add(pred, r...); err != nil {
				return nil, 0, err
			}
		}
	}
	methods := make([]string, 0, len(parts))
	for m := range parts {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	steps := 0
	for _, m := range methods {
		db, n, err := reference(parts[m], localFlowRules, cap-steps, true)
		steps += n
		if err != nil {
			return nil, steps, fmt.Errorf("method-local reference %s: %w", m, err)
		}
		for pred := range output {
			for _, r := range db.Facts(pred) {
				if err := seed.Add(pred, r...); err != nil {
					return nil, steps, err
				}
			}
		}
	}
	db, n, err := reference(seed, clauseSource(global), cap-steps, true)
	return db, steps + n, err
}

func clauseSource(cs []clause) string {
	writeAtom := func(a atom) string {
		var b strings.Builder
		if a.negative {
			b.WriteString("(not ")
		}
		b.WriteString("(" + a.pred)
		for _, n := range a.terms {
			b.WriteByte(' ')
			if n.quoted {
				b.WriteString(strconv.Quote(n.text))
			} else {
				b.WriteString(n.text)
			}
		}
		b.WriteByte(')')
		if a.negative {
			b.WriteByte(')')
		}
		return b.String()
	}
	var b strings.Builder
	for i, c := range cs {
		fmt.Fprintf(&b, "(rule reference%d 0 (head %s) (base", i, writeAtom(c.head))
		for _, a := range c.body {
			b.WriteByte(' ')
			b.WriteString(writeAtom(a))
		}
		b.WriteByte(')')
		if c.bound >= 0 {
			fmt.Fprintf(&b, " (bound depth %d)", c.bound)
		}
		b.WriteString(")\n")
	}
	return b.String()
}
