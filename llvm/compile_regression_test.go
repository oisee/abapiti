package llvm

import (
	"os"
	"strings"
	"testing"
)

func TestReportMemoryCallsUseMethods(t *testing.T) {
	source, err := os.ReadFile("testdata/corpus.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := Parse(string(source))
	if err != nil {
		t.Fatal(err)
	}
	src := compileABAP(t, mod, "zcl_corpus")
	for _, helper := range []string{"mem_ld_i32", "mem_st_i32"} {
		if !strings.Contains(src, "CLASS-METHODS "+helper+" IMPORTING ") ||
			!strings.Contains(src, "METHOD "+helper+".") || strings.Contains(src, "FORM "+helper+" USING ") {
			t.Fatalf("memory call must resolve to method %s", helper)
		}
	}
}
