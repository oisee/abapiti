package rewrite

import (
	"fmt"
)

// Evaluate grows db to a stratified fixed point. Each join after the initial
// round consumes at least one delta relation (semi-naive evaluation). Minimum
// proof depths are memoised too, so a later shorter proof can unlock a bound.
func Evaluate(db *DB, rules *Rules) error {
	levels := map[string]int{}
	arity := map[string]int{}
	for p, t := range db.tables {
		arity[p] = t.arity
		levels[p] = 0
	}
	for _, c := range rules.clauses {
		if comparison(c.head.pred) {
			return fmt.Errorf("%s: comparison in head", c.name)
		}
		bound := map[string]bool{}
		for _, a := range c.body {
			if !a.negative && !comparison(a.pred) {
				for _, t := range a.args {
					if t.variable {
						bound[t.value] = true
					}
				}
			}
		}
		for _, a := range append([]atom{c.head}, c.body...) {
			if comparison(a.pred) && len(a.args) != 2 {
				return fmt.Errorf("%s needs two terms", a.pred)
			}
			if n, ok := arity[a.pred]; ok && n != len(a.args) {
				return fmt.Errorf("%s: inconsistent arity", a.pred)
			}
			arity[a.pred] = len(a.args)
			levels[a.pred] = 0
			for _, t := range a.args {
				if (a.negative || comparison(a.pred) || a.pred == c.head.pred) && t.variable && !bound[t.value] {
					return fmt.Errorf("%s: unbound %s", c.name, t.value)
				}
			}
		}
		for _, t := range c.head.args {
			if t.wild {
				return fmt.Errorf("%s: wildcard in head", c.name)
			}
		}
	}
	// Difference constraints reject a dependency cycle through negation.
	for round := 0; round <= len(levels); round++ {
		changed := false
		for _, c := range rules.clauses {
			for _, a := range c.body {
				n := levels[a.pred]
				if a.negative {
					n++
				}
				if levels[c.head.pred] < n {
					levels[c.head.pred] = n
					changed = true
				}
			}
		}
		if !changed {
			break
		}
		if round == len(levels) {
			return fmt.Errorf("unstratified negation")
		}
	}
	plans := make([][]*joinPlan, len(rules.clauses))
	for i, c := range rules.clauses {
		for pivot, a := range c.body {
			if !a.negative && !comparison(a.pred) {
				plans[i] = append(plans[i], planJoin(db, c, pivot, nil))
			}
		}
		if len(plans[i]) == 0 {
			plans[i] = append(plans[i], planJoin(db, c, -1, nil))
		}
	}
	maxLevel := 0
	for _, n := range levels {
		if n > maxLevel {
			maxLevel = n
		}
	}
	for level := 0; level <= maxLevel; level++ {
		delta := db
		first := true
		for {
			next := db.empty()
			for ci, c := range rules.clauses {
				if levels[c.head.pred] != level {
					continue
				}
				var emitErr error
				emit := func(args Tuple, depth int) {
					if emitErr == nil && (db.evaluateRegion == nil || db.evaluateRegion(c.head.pred, args)) {
						_, emitErr = next.put(c.head.pred, args, depth)
					}
				}
				for pi, plan := range plans[ci] {
					if first && pi > 0 {
						break
					}
					if plan.pivot >= 0 || first {
						plan.run(db, delta, nil, emit)
					}
				}
				if emitErr != nil {
					return emitErr
				}
			}
			fresh := db.empty()
			for _, p := range next.Predicates() {
				for _, r := range next.tables[p].rows {
					changed, e := db.put(p, r.args, r.depth)
					if e != nil {
						return e
					}
					if changed {
						if _, e := fresh.put(p, r.args, r.depth); e != nil {
							return e
						}
					}
				}
			}
			if len(fresh.tables) == 0 {
				break
			}
			delta = fresh
			first = false
		}
	}
	return nil
}
func matches(a atom, r *row, env map[string]string) (map[string]string, bool) {
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	for i, t := range a.args {
		if t.wild {
			continue
		}
		v := t.value
		if t.variable {
			if old, ok := out[v]; ok {
				if old != r.args[i] {
					return nil, false
				}
			} else {
				out[v] = r.args[i]
			}
		} else if v != r.args[i] {
			return nil, false
		}
	}
	return out, true
}
func candidates(d *DB, a atom, env map[string]string) []*row {
	t := d.tables[a.pred]
	if t == nil {
		return nil
	}
	var k packedTuple
	mask := uint64(0)
	for col, term := range a.args {
		if col >= 8 || term.wild {
			continue
		}
		value := term.value
		bound := !term.variable
		if term.variable {
			value, bound = env[value]
		}
		if bound {
			mask |= 1 << col
			k.ids[col] = d.symbols.ids[value]
		}
	}
	if mask == 0 {
		return t.rows
	}
	return t.ensureIndex(mask)[k]
}

func comparison(p string) bool { return p == "le" || p == "neq" || p == "contains" }
