package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Unlike the optional upstream differential, emission always runs in CI.
func TestChaiEmission(t *testing.T) {
	files, names, err := EmitNamed(chaiFixture())
	if err != nil {
		t.Fatal(err)
	}
	runtime := files[names.Get("runtime.chai")+".clas.abap"]
	for _, part := range []string{"METHOD deepEqual.", "METHOD deepInclude.", "METHOD arrayInclude.", "METHOD members.", "METHOD throwMatch.", "METHOD box_ref.", "READ TABLE seen WITH KEY", "RESPECTING CASE", "RAISE EXCEPTION error", "p0->chai_zero_sign = 0"} {
		if !strings.Contains(runtime, part) {
			t.Fatal("missing Chai semantic boundary", part)
		}
	}
	if strings.Contains(runtime, " CS ") {
		t.Fatal("ABAP CS is not case-sensitive JS substring matching")
	}
	for path, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s: overlong ABAP line", path)
			}
		}
	}
	// Chai machinery must not alter the output of ordinary programs.
	ordinary, err := Emit(&hir.Program{Classes: []*hir.Class{{Name: "Plain", Methods: []*hir.Method{method("run", i32, ret(lit(1)))}}}})
	if err != nil {
		t.Fatal(err)
	}
	for path, src := range ordinary {
		if strings.Contains(path, "chai") || strings.Contains(src, "chai_zero_sign") {
			t.Fatal("Chai runtime escaped its usage gate")
		}
	}
}
