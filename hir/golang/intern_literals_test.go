package golang

import (
	"strings"
	"testing"
)

// Only real str("…") calls are hoisted: raw strings, string literals and
// comments that merely contain the text keep their content.
func TestInternStrLiteralsOnlyCalls(t *testing.T) {
	src := "package main\n" +
		"func str(s string) jsString { return jsString(s) }\n" +
		"var raw = `str(\"raw\")`\n" +
		"var quoted = \"str(\\\"quoted\\\")\"\n" +
		"// str(\"comment\")\n" +
		"var call = str(\"call\")\n" +
		"var again = x.str(\"method\")\n"
	files := map[string]string{"hir.go": src}
	vars := internStrLiterals(files, []string{"hir.go"})
	out := files["hir.go"]
	for _, kept := range []string{"`str(\"raw\")`", "\"str(\\\"quoted\\\")\"", "// str(\"comment\")", "x.str(\"method\")", "func str(s string)"} {
		if !strings.Contains(out, kept) {
			t.Errorf("rewrote %s:\n%s", kept, out)
		}
	}
	if strings.Contains(out, "str(\"call\")") || !strings.Contains(vars, "= str(\"call\")") || strings.Count(vars, "\n") != 1 {
		t.Fatalf("call not hoisted exactly once:\n%s\n%s", out, vars)
	}
}
