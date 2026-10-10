package rewrite

import "testing"

func TestRewriteDependencies(t *testing.T) {
	b, err := ruleFiles.ReadFile("rules/inline.grace")
	if err != nil {
		t.Fatal(err)
	}
	_, rules, err := Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	selected, needed, err := rewriteDependencies(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"node", "dispatch", "defined", "static", "inline_allowed", "inline_path"} {
		if !needed[p] {
			t.Fatalf("missing dependency %s", p)
		}
	}
	for _, p := range []string{"pure", "escapes", "receivers", "calls", "inline_template"} {
		if needed[p] {
			t.Fatalf("unused dependency %s", p)
		}
	}
	if len(selected.clauses) != len(rules.clauses) {
		t.Fatal("inline selected unrelated analysis rules")
	}
	_, custom, err := Parse(`(grace pure-node 0 (match (node ?s lit)) (where (pure ?m) (not (escapes ?m ?s))) (action (replace ?s ?s)))`)
	if err != nil {
		t.Fatal(err)
	}
	_, needed, err = rewriteDependencies(custom)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"pure", "escapes", "impure", "receivers", "calls", "dispatch"} {
		if !needed[p] {
			t.Fatalf("missing transitive dependency %s", p)
		}
	}
}
