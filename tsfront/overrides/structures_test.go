package overrides

import (
	"os"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStructuresOverridesPinTheirSource(t *testing.T) {
	raw, err := os.ReadFile("../testdata/stmts/src/abap/3_structures/structure_parser.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, entry := range structuresOverrides() {
		name := strings.TrimPrefix(entry.Key.Symbol, "StructureParser.")
		start := strings.Index(source, "private static "+name+"(")
		if start < 0 {
			t.Fatal("missing", name)
		}
		end := start + strings.Index(source[start:], "\n  }") + len("\n  }")
		if _, ok, err := Abaplint().Lookup(entry.Key, source[start:end], entry.Key.File); err != nil || !ok {
			t.Fatal(name, ok, err)
		}
		if _, _, err := Abaplint().Lookup(entry.Key, source[start:end]+" ", entry.Key.File); err == nil {
			t.Fatal("stale structure override accepted", name)
		}
		if entry.Result != nil && !entry.Result().Equal(hir.Ref("src/abap/3_structures/structure_result.ts.IStructureResult")) {
			t.Fatal("result shape changed")
		}
	}
}
