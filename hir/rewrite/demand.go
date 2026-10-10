package rewrite

// rewriteDependencies selects the transitive relation dependency closure,
// including negative guards. Analysis remains available to user rewrite rules.
func rewriteDependencies(rules *Rules) (*Rules, map[string]bool, error) {
	source, err := ruleFiles.ReadFile("rules/analysis.grace")
	if err != nil {
		return nil, nil, err
	}
	_, analysis, err := Parse(string(source))
	if err != nil {
		return nil, nil, err
	}
	all := append(append([]clause{}, analysis.clauses...), rules.clauses...)
	needed := map[string]bool{}
	for _, r := range rules.rewrites {
		needed[r.match.pred] = true
		for _, a := range r.where {
			if !comparison(a.pred) {
				needed[a.pred] = true
			}
		}
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
	selected := &Rules{rewrites: rules.rewrites}
	for _, c := range all {
		if needed[c.head.pred] {
			selected.clauses = append(selected.clauses, c)
		}
	}
	return selected, needed, nil
}
