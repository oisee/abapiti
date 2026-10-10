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
	selected, needed := grace.SelectDemand([]*Rules{analysis, rules}, rules.RewriteGoals())
	return selected, needed, nil
}
