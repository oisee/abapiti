package wasm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoUnknownTrapClass: CX_SY_PROGRAM_ERROR does not exist on SAP or on
// open-steamgate, so a class that raises it does not activate at all. No
// backend may emit it, for any module in testdata (QuickJS included, which
// exercises unreachable on every path).
func TestNoUnknownTrapClass(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.wasm"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no testdata modules: %v", err)
	}
	for _, f := range files {
		bin, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		mod, err := Parse(bin)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		outs := map[string]string{"class": Compile(mod, "zcl_trap_check")}
		for _, b := range []BackendKind{BackendFUGR, BackendHybrid} {
			r := CompileWith(mod, "ZTRAP", b, 0)
			for name, src := range r.Files {
				outs[name] = src
			}
		}
		mc := mustCompileMultiClass(t, mod, "zcl_trap", 0)
		outs["multi-main"] = mc.MainClass
		outs["multi-runtime"] = mc.RuntimeClass
		for name, src := range mc.ChunkClasses {
			outs[name] = src
		}
		for name, src := range outs {
			if n := strings.Count(strings.ToLower(src), "cx_sy_program_error"); n > 0 {
				t.Errorf("%s %s: %d references to cx_sy_program_error", filepath.Base(f), name, n)
			}
		}
	}
}
