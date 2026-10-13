package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Unlike the optional upstream differential, emission always runs in CI.
func TestChaiEmission(t *testing.T) {
	files, names, err := emitABAPNamed(t, chaiFixture())
	if err != nil {
		t.Fatal(err)
	}
	runtime := files[names.Get("runtime.chai")+".clas.abap"]
	for _, part := range []string{"METHOD deepEqual.", "METHOD deepInclude.", "METHOD arrayInclude.", "METHOD members.", "METHOD throwMatch.", "METHOD box_ref.", "READ TABLE seen WITH KEY", "FIND FIRST OCCURRENCE OF p1", "RAISE EXCEPTION error", "p0->chai_zero_sign = 0"} {
		if !strings.Contains(runtime, part) {
			t.Fatal("missing Chai semantic boundary", part)
		}
	}
	// Plain FIND defaults to case-sensitive substring matching on ABAP.
	if strings.Contains(runtime, " CS ") || strings.Contains(runtime, "IGNORING CASE") {
		t.Fatal("ABAP CS is not case-sensitive JS substring matching")
	}
	if strings.Contains(runtime, "CATCH cx_root") || !strings.Contains(runtime, "CATCH "+names.Get("exception.ts_throw")) {
		t.Fatal("Chai must catch translated payloads and let native faults propagate")
	}
	for path, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s: overlong ABAP line", path)
			}
		}
	}
	// Chai machinery must not alter the output of ordinary programs.
	ordinary, err := emitABAP(t, &hir.Program{Classes: []*hir.Class{{Name: "Plain", Methods: []*hir.Method{method("run", i32, ret(lit(1)))}}}})
	if err != nil {
		t.Fatal(err)
	}
	for path, src := range ordinary {
		if strings.Contains(path, "chai") || strings.Contains(src, "chai_zero_sign") {
			t.Fatal("Chai runtime escaped its usage gate")
		}
	}
}
