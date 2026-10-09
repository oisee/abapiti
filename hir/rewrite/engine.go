package rewrite

import (
	"fmt"
	"strconv"
	"strings"
)

// Evaluate grows db to a stratified fixed point. Each join after the initial
// round consumes at least one delta relation (semi-naive evaluation). Minimum
// proof depths are memoised too, so a later shorter proof can unlock a bound.
func Evaluate(db *DB, rules *Rules) error {
	levels := map[string]int{}
	arity := map[string]int{}
	for p, t := range db.tables {
		arity[p] = len(t.index)
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
			next := NewDB()
			for _, c := range rules.clauses {
				if levels[c.head.pred] != level {
					continue
				}
				positive := false
				for pivot, a := range c.body {
					if a.negative || comparison(a.pred) {
						continue
					}
					positive = true
					join(db, delta, c, pivot, 0, map[string]string{}, 0, func(args Tuple, depth int) { next.put(c.head.pred, args, depth) })
				}
				if !positive && first {
					join(db, delta, c, -1, 0, map[string]string{}, 0, func(args Tuple, depth int) { next.put(c.head.pred, args, depth) })
				}
			}
			fresh := NewDB()
			for _, p := range next.Predicates() {
				for _, r := range next.tables[p].rows {
					changed, e := db.put(p, r.args, r.depth)
					if e != nil {
						return e
					}
					if changed {
						fresh.put(p, r.args, r.depth)
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
	out := t.rows
	for i, v := range a.args {
		s, ok := env[v.value]
		if !v.variable && !v.wild {
			s = v.value
			ok = true
		}
		if ok && len(t.index[i][s]) < len(out) {
			out = t.index[i][s]
		}
	}
	return out
}
func join(db, delta *DB, c clause, pivot, pos int, env map[string]string, depth int, emit func(Tuple, int)) {
	if pos == len(c.body) {
		// Negatives are checked after positive bindings, independently of source order.
		for _, a := range c.body {
			if comparison(a.pred) {
				if !compare(a, env) {
					return
				}
				continue
			}
			if a.negative {
				for _, r := range candidates(db, a, env) {
					if _, ok := matches(a, r, env); ok {
						return
					}
				}
			}
		}
		depth++
		if c.bound >= 0 && depth > c.bound {
			return
		}
		var args Tuple
		for _, t := range c.head.args {
			v := t.value
			if t.variable {
				v = env[v]
			}
			args = append(args, v)
		}
		emit(args, depth)
		return
	}
	a := c.body[pos]
	if a.negative || comparison(a.pred) {
		join(db, delta, c, pivot, pos+1, env, depth, emit)
		return
	}
	source := db
	if pos == pivot {
		source = delta
	}
	for _, r := range candidates(source, a, env) {
		if e, ok := matches(a, r, env); ok {
			n := depth
			if r.depth > n {
				n = r.depth
			}
			join(db, delta, c, pivot, pos+1, e, n, emit)
		}
	}
}

func comparison(p string) bool { return p == "le" || p == "neq" || p == "contains" }
func compare(a atom, env map[string]string) bool {
	if len(a.args) != 2 {
		return false
	}
	v := make([]string, 2)
	for i, t := range a.args {
		v[i] = t.value
		if t.variable {
			var ok bool
			v[i], ok = env[t.value]
			if !ok {
				return false
			}
		}
		if t.wild {
			return false
		}
	}
	var result bool
	switch a.pred {
	case "contains":
		result = strings.Contains(v[0], v[1])
	case "neq":
		result = v[0] != v[1]
	case "le":
		x, e := strconv.Atoi(v[0])
		y, f := strconv.Atoi(v[1])
		if e != nil || f != nil {
			return false
		}
		result = x <= y
	default:
		return false
	}
	if a.negative {
		result = !result
	}
	return result
}
