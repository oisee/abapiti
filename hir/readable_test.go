package hir

import (
	"strings"
	"testing"
)

// Readable names: prefixes per kind, a layer tag only where two classes
// share a name, harness names never taken, deterministic, within 30
// characters.
func TestReadableNames(t *testing.T) {
	p := &Program{
		Classes: []*Class{
			{Name: "src/abap/1_lexer/lexer.ts.Lexer", Methods: []*Method{{Name: "getTokens", Params: []Param{{Name: "file"}}}}},
			{Name: "src/abap/2_statements/expressions/method_source.ts.MethodSource"},
			{Name: "src/abap/5_syntax/expressions/method_source.ts.MethodSource"},
			{Name: "src/abap/2_statements/combi.ts.FailCombinatorError", Super: "builtin.Error"},
			{Name: "harness/registry_osg.ts.RegistryOSG"},
			{Name: "src/rules/a_very_long_rule_name_for_testing.ts.AVeryLongRuleNameForTestingPurposesOnly"},
			{Name: "src/config.ts.IConfig"},
			{Name: "src/rules/align_parameters.ts.ICandidate"},
			{Name: "src/issue.ts.Issue"},
			{Name: "src/config.ts.Config"},
			{Name: "src/objects/iac_binary_data.ts.IACBinaryData"},
		},
		Interfaces: []*Interface{{Name: "src/abap/2_statements/statement_runnable.ts.IStatementRunnable"}},
	}
	n := NewReadableNames(p, "LNT")
	want := map[string]string{
		"src/abap/1_lexer/lexer.ts.Lexer":                                 "zcl_lnt_lexer",
		"src/abap/2_statements/expressions/method_source.ts.MethodSource": "zcl_lnt_st_method_source",
		"src/abap/5_syntax/expressions/method_source.ts.MethodSource":     "zcl_lnt_sy_method_source",
		"src/abap/2_statements/combi.ts.FailCombinatorError":              "zcx_lnt_fail_combinator_error",
		"src/abap/2_statements/statement_runnable.ts.IStatementRunnable":  "zif_lnt_statement_runnable",
		"src/config.ts.IConfig":                                           "zcl_lnt_iconfig",
		"src/config.ts.Config":                                            "zcl_lnt_config",
		"src/rules/align_parameters.ts.ICandidate":                        "zcl_lnt_candidate",
		"src/objects/iac_binary_data.ts.IACBinaryData":                    "zcl_lnt_iac_binary_data",
		"src/issue.ts.Issue":                                              "zcl_lnt_issue",
		"member.getTokens":                                                "get_tokens",
		"param.file":                                                      "i_file",
	}
	for id, w := range want {
		if got := n.Get(id); got != w {
			t.Errorf("%s: %s, want %s", id, got, w)
		}
	}
	if got := n.Get("harness/registry_osg.ts.RegistryOSG"); got == "zcl_lnt_registry_osg" {
		t.Errorf("translated class took the harness name %s", got)
	}
	long := n.Get("src/rules/a_very_long_rule_name_for_testing.ts.AVeryLongRuleNameForTestingPurposesOnly")
	if len(long) > 30 || !strings.HasPrefix(long, "zcl_lnt_") {
		t.Errorf("long name %q", long)
	}
	if again := NewReadableNames(p, "LNT").Get("src/abap/5_syntax/expressions/method_source.ts.MethodSource"); again != "zcl_lnt_sy_method_source" {
		t.Errorf("not deterministic: %s", again)
	}
	if fb := n.Get("runtime.array<i32>"); !strings.HasPrefix(fb, "zlnt_") || len(fb) > 30 {
		t.Errorf("fallback %q", fb)
	}
}
