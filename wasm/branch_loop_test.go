package wasm

import (
	"fmt"
	"strings"
	"testing"
)

// buildBranchLoopWasm keeps every branch target explicit rather than relying on
// a C optimizer to choose a particular control-flow shape.
func buildBranchLoopWasm() []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Params: []ValType{ValI32}, Results: []ValType{ValI32}}}))
	names := []string{"sum_if", "sum_br", "nested", "table", "if_block", "fallthrough", "nested_if", "nested_table"}
	var exports []Export
	var types []int
	for i, name := range names {
		exports = append(exports, Export{Name: name, Kind: 0, Index: i})
		types = append(types, 0)
	}
	w.addSection(3, buildFuncSection(types))
	w.addSection(7, buildExportSection(exports))
	// Local 1 counts iterations; local 2 accumulates the sum.
	inc := []byte{OpLocalGet, 1, OpI32Const, 1, OpI32Add, OpLocalSet, 1}
	sum := []byte{OpLocalGet, 2, OpLocalGet, 1, OpI32Add, OpLocalSet, 2}
	less := []byte{OpLocalGet, 1, OpLocalGet, 0, OpI32LtS}
	done := []byte{OpLocalGet, 1, OpLocalGet, 0, OpI32GeS}
	join := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	codes := [][]byte{
		join([]byte{OpLoop, 0x40}, inc, sum, less, []byte{OpBrIf, 0, OpEnd, OpLocalGet, 2}),
		join([]byte{OpBlock, 0x40, OpLoop, 0x40}, done, []byte{OpBrIf, 1}, inc, sum, []byte{OpBr, 0, OpEnd, OpEnd, OpLocalGet, 2}),
		// br 1 restarts the outer loop; br 2 leaves both loops and the block.
		join([]byte{OpBlock, 0x40, OpLoop, 0x40}, inc, []byte{OpLoop, 0x40}, done, []byte{OpBrIf, 2, OpBr, 1, OpEnd, OpI32Const, 42, OpLocalSet, 1, OpEnd, OpEnd, OpLocalGet, 1}),
		join([]byte{OpBlock, 0x40, OpLoop, 0x40}, inc, sum, less, []byte{OpI32Eqz, OpBrTable, 1, 0, 1, OpEnd, OpEnd, OpLocalGet, 2}),
		// Exiting a block/if inside a loop must resume after that label.
		join([]byte{OpBlock, 0x40, OpLoop, 0x40, OpI32Const, 1, OpIf, 0x40, OpBr, 0, OpEnd, OpBlock, 0x40, OpBr, 0, OpEnd}, inc, sum, done, []byte{OpBrIf, 1, OpBr, 0, OpEnd, OpEnd, OpLocalGet, 2}),
		join([]byte{OpLoop, 0x40}, inc, []byte{OpEnd, OpLocalGet, 1}),
		join([]byte{OpBlock, 0x40, OpLoop, 0x40}, inc, []byte{OpLoop, 0x40}, less, []byte{OpBrIf, 1, OpBr, 2, OpEnd, OpEnd, OpEnd, OpLocalGet, 1}),
		join([]byte{OpBlock, 0x40, OpLoop, 0x40}, inc, []byte{OpLoop, 0x40}, less, []byte{OpI32Eqz, OpBrTable, 1, 1, 2, OpEnd, OpEnd, OpEnd, OpLocalGet, 1}),
	}
	var bodies [][]byte
	for _, code := range codes {
		bodies = append(bodies, buildFuncBody([]ValType{ValI32, ValI32}, code))
	}
	w.addSection(10, buildCodeSection(bodies))
	return w.bytes()
}

var branchLoopCases = []osdCase{
	{"sum_if", []int32{5}}, {"sum_if", []int32{10}},
	{"sum_br", []int32{0}}, {"sum_br", []int32{5}},
	{"nested", []int32{5}}, {"nested", []int32{10}},
	{"table", []int32{5}}, {"table", []int32{10}},
	{"if_block", []int32{5}}, {"fallthrough", []int32{5}},
	{"nested_if", []int32{5}}, {"nested_table", []int32{5}},
}

func TestBranchLoopCodegen(t *testing.T) {
	bin := buildBranchLoopWasm()
	want := []int32{15, 55, 0, 15, 5, 10, 15, 55, 15, 1, 5, 5}
	for i, got := range wazeroResults(t, bin, branchLoopCases) {
		if got != want[i] {
			t.Fatalf("fixture %s: got %d want %d", branchLoopCases[i].fn, got, want[i])
		}
	}
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
		t.Run(backend.String(), func(t *testing.T) {
			result := CompileWith(mod, "zcl_loop", backend, 80)
			var src string
			for _, file := range result.Files {
				src += file + "\n"
			}
			src = strings.Join(strings.Fields(src), " ")
			br := "lv_br"
			if backend != BackendClass {
				br = "br"
			}
			for _, pattern := range []string{
				"CONTINUE.",
				"WHEN 0. CONTINUE. WHEN OTHERS.",
				fmt.Sprintf("%s = -2. EXIT.", br),
				fmt.Sprintf("WHEN 0. %s = -2. EXIT. WHEN OTHERS.", br),
				fmt.Sprintf("IF %s = -1. %s = 0. CONTINUE. ENDIF.", br, br),
				fmt.Sprintf("ENDDO. IF %s > 0 AND %s <> 999.", br, br),
			} {
				if !strings.Contains(src, pattern) {
					t.Errorf("missing branch structure: %s", pattern)
				}
			}
			if backend == BackendClass {
				// The unconditional back edge must continue; EXIT is only fallthrough.
				if !strings.Contains(src, "CONTINUE. EXIT. ENDDO.") {
					t.Error("unconditional loop back edge does not continue")
				}
			}
		})
	}
	result := CompileMultiClass(mod, "zcl_loop", 2)
	var multi string
	for _, file := range result.ChunkClasses {
		multi += file
	}
	multi = strings.Join(strings.Fields(multi), " ")
	for _, pattern := range []string{
		"WHEN 0. CONTINUE. WHEN OTHERS.",
		"WHEN 0. lv_br = -2. EXIT. WHEN OTHERS.",
		"IF lv_br = -1. lv_br = 0. CONTINUE. ENDIF.",
		"ENDDO. IF lv_br > 0 AND lv_br <> 999.",
		"CONTINUE. EXIT. ENDDO.",
	} {
		if !strings.Contains(multi, pattern) {
			t.Errorf("multi-class missing branch structure: %s", pattern)
		}
	}
}
