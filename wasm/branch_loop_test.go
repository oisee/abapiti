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
	names := []string{"sum_if", "sum_br", "nested", "table", "if_block", "fallthrough", "nested_if", "nested_table", "value_if", "value_loop", "value_table"}
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
	codes = append(codes,
		[]byte{OpBlock, 0x7f, OpI32Const, 5, OpLocalGet, 0, OpLocalGet, 0, OpI32Const, 3, OpI32GtS, OpBrIf, 0, OpDrop, OpEnd},
		[]byte{OpBlock, 0x7f, OpI32Const, 7, OpLoop, 0x40, OpLocalGet, 0, OpI32Const, 10, OpI32Mul, OpLocalGet, 0, OpI32Const, 3, OpI32GtS, OpBrIf, 1, OpDrop, OpEnd, OpEnd},
		[]byte{OpBlock, 0x7f, OpI32Const, 7, OpBlock, 0x7f, OpLocalGet, 0, OpI32Const, 10, OpI32Mul, OpLocalGet, 0, OpBrTable, 1, 0, 1, OpEnd, OpLocalSet, 1, OpDrop, OpLocalGet, 1, OpEnd},
	)
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
	{"value_if", []int32{9}}, {"value_if", []int32{2}},
	{"value_loop", []int32{5}}, {"value_loop", []int32{2}},
	{"value_table", []int32{0}}, {"value_table", []int32{5}},
}

func TestBranchLoopCodegen(t *testing.T) {
	bin := buildBranchLoopWasm()
	want := []int32{15, 55, 0, 15, 5, 10, 15, 55, 15, 1, 5, 5, 9, 5, 50, 7, 0, 50}
	for i, got := range wazeroResults(t, bin, branchLoopCases) {
		if got.trap || got.value != want[i] {
			t.Fatalf("fixture %s: got %+v want %d", branchLoopCases[i].fn, got, want[i])
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

// Assert complete extracted-loop bodies and propagation, including the sign
// update. These checks catch bounded-loop and negative-depth decrement mutants.
func TestExtractedBranchSemantics(t *testing.T) {
	mod, err := Parse(buildBranchLoopWasm())
	if err != nil {
		t.Fatal(err)
	}
	indices := make([]int, len(mod.Functions))
	for i := range indices {
		indices[i] = i
	}
	forms, methods := emitFUGRInclude(mod, indices, nil, "ZLOOP", false)
	for _, bm := range methods {
		body := strings.Join(strings.Fields(bm.body), " ")
		if bm.isLoop && !strings.HasPrefix(body, "DO.") {
			t.Errorf("%s: extracted loop must be unbounded: %s", bm.name, body)
		}
		want := "IF br > 0 AND br <> 999. br = br - 1. ELSEIF br < 0. br = br + 1. ENDIF."
		if !strings.Contains(body, want) {
			t.Errorf("%s: incomplete propagation: %s", bm.name, body)
		}
	}
	for _, name := range []string{"value_if", "value_loop", "value_table"} {
		var bodies string
		for _, bm := range methods {
			if bm.parentFunc.ExportName == name {
				bodies += bm.body
			}
		}
		if !strings.Contains(strings.Join(strings.Fields(bodies), " "), "s0 = s1.") {
			t.Errorf("%s: extracted branch value missing from target slot", name)
		}
	}
	forms = strings.Join(strings.Fields(forms), " ")
	for _, name := range []string{"value_if", "value_loop", "value_table"} {
		start := strings.Index(forms, "FORM "+name+" ")
		body := forms[start:]
		body = body[:strings.Index(body, "ENDFORM.")]
		if !strings.Contains(body, "rv = g=>s0.") {
			t.Errorf("%s: block result dropped: %s", name, body)
		}
	}
}

func TestBranchValueSlots(t *testing.T) {
	mod, err := Parse(buildBranchLoopWasm())
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Join(strings.Fields(Compile(mod, "zcl_values")), " ")
	for _, name := range []string{"value_if", "value_loop", "value_table"} {
		start := strings.Index(src, "METHOD "+name+".")
		body := src[start:]
		body = body[:strings.Index(body, "ENDMETHOD.")]
		if !strings.Contains(body, "s0 = s1.") {
			t.Errorf("%s: branch value missing from target slot: %s", name, body)
		}
	}
}

func TestBlockTypeDecoding(t *testing.T) {
	types := make([]FuncType, 65)
	types[0] = FuncType{Results: []ValType{ValI32}}
	types[64] = FuncType{Params: []ValType{ValI32}, Results: []ValType{ValI64, ValI32}}
	c := &compiler{mod: &Module{Types: types}}
	if got := c.blockType(64); len(got.Params) != 1 || len(got.Results) != 2 {
		t.Fatalf("type index 64 treated as empty: %+v", got)
	}
	for _, test := range []struct {
		encoded int
		results []ValType
	}{
		{-64, nil}, {-1, []ValType{ValI32}}, {-2, []ValType{ValI64}},
		{-3, []ValType{ValF32}}, {-4, []ValType{ValF64}}, {0, []ValType{ValI32}},
	} {
		got := c.blockType(test.encoded).Results
		if fmt.Sprint(got) != fmt.Sprint(test.results) {
			t.Errorf("%d: got %v want %v", test.encoded, got, test.results)
		}
	}
}

func TestTypedBranchValueSlots(t *testing.T) {
	for _, typ := range []struct {
		block  byte
		op     byte
		value  []byte
		suffix string
	}{
		{0x7e, OpI64Const, []byte{9}, "_i64"},
		{0x7d, OpF32Const, []byte{0, 0, 16, 65}, "_f"},
		{0x7c, OpF64Const, []byte{0, 0, 0, 0, 0, 0, 34, 64}, "_f"},
	} {
		w := newWasmBuilder()
		w.addSection(1, buildTypeSection([]FuncType{{Results: []ValType{ValType(typ.block)}}}))
		w.addSection(3, buildFuncSection([]int{0}))
		w.addSection(7, buildExportSection([]Export{{Name: "value", Kind: 0, Index: 0}}))
		code := []byte{OpBlock, typ.block, OpI32Const, 5, typ.op}
		code = append(code, typ.value...)
		code = append(code, OpBr, 0, OpEnd)
		w.addSection(10, buildCodeSection([][]byte{buildFuncBody(nil, code)}))
		mod, err := Parse(w.bytes())
		if err != nil {
			t.Fatal(err)
		}
		src := strings.Join(strings.Fields(Compile(mod, "zcl_typed")), " ")
		for _, want := range []string{"s0" + typ.suffix + " = s1" + typ.suffix + ".", "rv = s0" + typ.suffix + "."} {
			if !strings.Contains(src, want) {
				t.Errorf("type %x: missing %s", typ.block, want)
			}
		}
	}
}
