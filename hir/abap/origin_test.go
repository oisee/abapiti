package abap

import "testing"

func TestOriginComment(t *testing.T) {
	cases := []struct{ id, source, want string }{
		{"src/abap/1_lexer/lexer.ts.Lexer", "/tmp/x/src/abap/1_lexer/lexer.ts:56:1", "* TS: src/abap/1_lexer/lexer.ts:56:1 Lexer\n"},
		{"union.abc", "", ""},
		{"harness/registry_run.ts.RegistryRun", "/b/harness/registry_run.ts:3:1", "* TS: harness/registry_run.ts:3:1 RegistryRun\n"},
	}
	for _, c := range cases {
		if got := originComment(c.id, c.source); got != c.want {
			t.Errorf("originComment(%q, %q) = %q, want %q", c.id, c.source, got, c.want)
		}
	}
}
