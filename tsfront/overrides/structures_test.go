package overrides

import (
	"os"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStructuresOverridesPinTheirSource(t *testing.T) {
	for _, entry := range structuresOverrides() {
		raw, err := os.ReadFile("../testdata/stmts/" + entry.Key.File)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		method := strings.TrimPrefix(entry.Key.Symbol, "StructureParser.")
		start := strings.Index(source, "private static "+method+"(")
		if start < 0 && strings.Contains(entry.Key.Symbol, ".") {
			owner := strings.SplitN(entry.Key.Symbol, ".", 2)[0]
			classStart := strings.Index(source, "class "+owner)
			if classStart >= 0 {
				start = strings.Index(source[classStart:], "public run(") + classStart
			}
		}
		if start < 0 {
			t.Fatal("missing", entry.Key.Symbol)
		}
		end := start + strings.Index(source[start:], "\n  }") + len("\n  }")
		if _, ok, err := Abaplint().Lookup(entry.Key, source[start:end], entry.Key.File); err != nil || !ok {
			t.Fatal(entry.Key.Symbol, ok, err)
		}
		if _, _, err := Abaplint().Lookup(entry.Key, source[start:end]+" ", entry.Key.File); err == nil {
			t.Fatal("stale structure override accepted", entry.Key.Symbol)
		}
		if entry.Result != nil && !entry.Result().Equal(hir.Ref("src/abap/3_structures/structure_result.ts.IStructureResult")) {
			t.Fatal("result shape changed")
		}
	}
}
