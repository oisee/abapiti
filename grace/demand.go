package grace

// SelectDemand selects the transitive dependency closure of goal relations,
// including negative guards. Sets are concatenated in order without reordering
// their clauses. The returned relation set also includes required base facts.
func SelectDemand(sets []*Rules, goals []string) (*Rules, map[string]bool) {
	var all []clause
	for _, rules := range sets {
		all = append(all, rules.clauses...)
	}
	needed := map[string]bool{}
	for _, goal := range goals {
		needed[goal] = true
	}
	for changed := true; changed; {
		changed = false
		for _, c := range all {
			if !needed[c.head.pred] {
				continue
			}
			for _, a := range c.body {
				if !comparison(a.pred) && !needed[a.pred] {
					needed[a.pred] = true
					changed = true
				}
			}
		}
	}
	selected := &Rules{}
	for _, c := range all {
		if needed[c.head.pred] {
			selected.clauses = append(selected.clauses, c)
		}
	}
	return selected, needed
}

// Heads returns clause head relation names in evaluation order, with repeats.
func (r *Rules) Heads() []string {
	out := make([]string, 0, len(r.clauses))
	for _, c := range r.clauses {
		out = append(out, c.head.pred)
	}
	return out
}

// RewriteGoals returns the relations used by rewrite matches and guards.
func (r *Rules) RewriteGoals() []string {
	var out []string
	for _, rr := range r.rewrites {
		out = append(out, rr.match.pred)
		for _, a := range rr.where {
			if !comparison(a.pred) {
				out = append(out, a.pred)
			}
		}
	}
	return out
}
