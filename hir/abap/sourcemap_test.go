package abap

import "testing"

func TestRelSource(t *testing.T) {
	for in, want := range map[string]string{
		"/tmp/abapiti-abaplint-1/src/abap/1_lexer/lexer.ts:64:3":                "src/abap/1_lexer/lexer.ts:64:3",
		"/tmp/abapiti-abaplint-1/harness/registry_run.ts:28:3":                  "harness/registry_run.ts:28:3",
		"/tmp/abapiti-abaplint-1/node_modules/fast-xml-parser/src/fxp.d.ts:9:1": "node_modules/fast-xml-parser/src/fxp.d.ts:9:1",
		"src/registry.ts:70:7": "src/registry.ts:70:7",
	} {
		if got := relSource(in); got != want {
			t.Errorf("relSource(%q) = %q, want %q", in, got, want)
		}
	}
}
