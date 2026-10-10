package rewrite

import "github.com/oisee/abapiti/grace"

// rewriteDependencies supplies the HIR analysis rules and rewrite goals.
func rewriteDependencies(rules *Rules) (*Rules, map[string]bool, error) {
	source, err := ruleFiles.ReadFile("rules/analysis.grace")
	if err != nil {
		return nil, nil, err
	}
	_, analysis, err := Parse(string(source))
	if err != nil {
		return nil, nil, err
	}
	sets := []*Rules{analysis, rules}
	goals := rules.RewriteGoals()
	if isStoreRules(rules) {
		sets = []*Rules{rules}
	}
	for _, rule := range rules.Rewrites() {
		if rule.Action() == "substitute-use" || rule.Action() == "remove-statement" {
			goals = append(goals, "next", "def", "use", "local_ref")
		}
	}
	selected, needed := grace.SelectDemand(sets, goals)
	return selected, needed, nil
}
