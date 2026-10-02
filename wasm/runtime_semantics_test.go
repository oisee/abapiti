package wasm

import (
	"strings"
	"testing"
)

// buildCallIndirectModule is a function-pointer table as clang lays it out:
// one element segment at offset 1, so slot 0 is null. Slots 1..3 hold
// (i32)->i32 functions, slot 4 a ()->i32 one, slot 5 an (i32)->i32 one
// declared under a second, identical type (call_indirect compares types
// structurally); the table has 6 slots. call(i, x) is call_indirect
// (i32)->i32 on slot i.
func buildCallIndirectModule() []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{
		{Params: []ValType{ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI32}, Results: []ValType{ValI32}},
		{Results: []ValType{ValI32}},
		{Params: []ValType{ValI32}, Results: []ValType{ValI32}},
	}))
	w.addSection(3, buildFuncSection([]int{0, 0, 0, 2, 1, 3}))
	w.addSection(4, []byte{1, 0x70, 0, 6}) // one funcref table, min 6
	w.addSection(7, buildExportSection([]Export{{Name: "call", Kind: 0, Index: 4}}))
	w.addSection(9, []byte{1, 0, OpI32Const, 1, OpEnd, 5, 0, 1, 2, 3, 5})
	w.addSection(10, buildCodeSection([][]byte{
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpI32Const, 1, OpI32Add}),
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpI32Const, 2, OpI32Mul}),
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpI32Const, 3, OpI32Sub}),
		buildFuncBody(nil, append([]byte{OpI32Const}, leb128s(99)...)),
		buildFuncBody(nil, []byte{OpLocalGet, 1, OpLocalGet, 0, OpCallIndirect, 0, 0}),
		buildFuncBody(nil, append(append([]byte{OpLocalGet, 0, OpI32Const}, leb128s(100)...), OpI32Add)),
	}))
	return w.bytes()
}

// helperData is the memory image the load helpers read.
var helperData = []byte{
	0xFF,       // 0: i8 -1 / u8 255
	0x05,       // 1: 5
	0x00, 0x80, // 2: i16 -32768 / u16 32768
	0xFF, 0x7F, // 4: 32767
	0x00, 0x00, 0x00, 0x80, // 6: i32 -2^31 / u32 2^31
	0xFF, 0xFF, 0xFF, 0x7F, // 10: 2^31-1
}

// buildHelperModule exercises runtime helpers: the sign-extending i64 loads
// (mem_ld_i64_ext) and the arithmetic right shifts (shr_s32, shr_s64).
// Every export is (i32[, i32]) -> i32; i64 results are wrapped, and the
// *_hi exports return the upper 32 bits (shr_u by 32).
func buildHelperModule() []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{
		{Params: []ValType{ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI32}, Results: []ValType{ValI32}},
	}))
	type fn struct {
		name string
		typ  int
		code []byte
	}
	load := func(op byte) []byte { return []byte{OpLocalGet, 0, op, 0, 0} }
	hi := []byte{OpI64Const, 32, OpI64ShrU}
	wrap := []byte{OpI32WrapI64}
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	x64 := []byte{OpLocalGet, 0, OpI64ExtendI32S}
	k64 := []byte{OpLocalGet, 1, OpI64ExtendI32S}
	funcs := []fn{
		{"l8s", 0, cat(load(0x30), wrap)},
		{"l8u", 0, cat(load(0x31), wrap)},
		{"l16s", 0, cat(load(0x32), wrap)},
		{"l16u", 0, cat(load(0x33), wrap)},
		{"l32s", 0, cat(load(0x34), wrap)},
		{"l32s_hi", 0, cat(load(0x34), hi, wrap)},
		{"l32u_hi", 0, cat(load(0x35), hi, wrap)},
		{"sar32", 1, []byte{OpLocalGet, 0, OpLocalGet, 1, OpI32ShrS}},
		{"sar64", 1, cat(x64, k64, []byte{OpI64ShrS}, wrap)},
		// (x << 32) >>s k, wrapped: the shift on a value beyond 32 bits.
		{"sar64_hi", 1, cat(x64, []byte{OpI64Const, 32, OpI64Shl}, k64, []byte{OpI64ShrS}, wrap)},
	}
	types := make([]int, len(funcs))
	exports := make([]Export, len(funcs))
	bodies := make([][]byte, len(funcs))
	for i, f := range funcs {
		types[i] = f.typ
		exports[i] = Export{Name: f.name, Kind: 0, Index: i}
		bodies[i] = buildFuncBody(nil, f.code)
	}
	w.addSection(3, buildFuncSection(types))
	w.addSection(5, []byte{1, 0, 1}) // one memory, min 1 page
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	data := append([]byte{1, 0, OpI32Const, 0, OpEnd}, leb128u(uint32(len(helperData)))...)
	w.addSection(11, append(data, helperData...))
	return w.bytes()
}

func compileClass(t *testing.T, bin []byte, class string) string {
	t.Helper()
	mod, err := Parse(bin)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Compile(mod, class)
}

// The element segment offset decides which slot a function lands in. The
// semantic check runs on OSD (osdModules "callind"); here the table layout.
func TestCallIndirectTableHonoursSegmentOffset(t *testing.T) {
	src := compileClass(t, buildCallIndirectModule(), "zcl_test_ci")
	var appends []string
	for _, line := range strings.Split(src, "\n") {
		for _, stmt := range strings.SplitAfter(line, ".") {
			if s := strings.TrimSpace(stmt); strings.HasPrefix(s, "APPEND ") {
				appends = append(appends, s)
			}
		}
	}
	want := []string{"APPEND -1 TO mt_tab0.", "APPEND 0 TO mt_tab0.", "APPEND 1 TO mt_tab0.", "APPEND 2 TO mt_tab0.", "APPEND 3 TO mt_tab0.", "APPEND 5 TO mt_tab0."}
	if strings.Join(appends, " ") != strings.Join(want, " ") {
		t.Fatalf("table init:\n got %q\nwant %q", appends, want)
	}
	// Slot 4 holds a ()->i32 function: dispatch for (i32)->i32 must not
	// reach it, but must reach function 5, whose type is identical.
	dispatch := src[strings.Index(src, "METHOD dispatch_t0."):]
	dispatch = dispatch[:strings.Index(dispatch, "ENDMETHOD.")]
	if strings.Contains(dispatch, "WHEN 3.") || !strings.Contains(dispatch, "WHEN 5.") ||
		!strings.Contains(dispatch, "WHEN OTHERS.") || !strings.Contains(dispatch, callIndirectTrap) {
		t.Fatalf("dispatch_t0 must call 0, 1, 2, 5 and trap otherwise:\n%s", dispatch)
	}
}

// An out-of-range index must trap, not reuse the previous target.
func TestCallIndirectRangeCheck(t *testing.T) {
	src := compileClass(t, buildCallIndirectModule(), "zcl_test_ci")
	if !strings.Contains(src, "< 0 OR ") || !strings.Contains(src, ">= lines( mt_tab0 ). "+callIndirectTrap+" ENDIF.") {
		t.Fatalf("call_indirect has no range check:\n%s", src)
	}
}

// A call_indirect with a type index outside the module used to emit only a
// comment, which the comment stripper then removed: the call vanished.
func TestCallIndirectInvalidTypeTraps(t *testing.T) {
	mod, err := Parse(buildCallIndirectModule())
	if err != nil {
		t.Fatal(err)
	}
	call := &mod.Functions[4]
	for i := range call.Code {
		if call.Code[i].Op == OpCallIndirect {
			call.Code[i].TypeIndex = 7
		}
	}
	src := Compile(mod, "zcl_test_ci")
	body := src[strings.Index(src, "METHOD call."):]
	body = body[:strings.Index(body, "ENDMETHOD.")]
	if !strings.Contains(body, callIndirectTrap) {
		t.Fatalf("invalid type index does not trap:\n%s", body)
	}
}

// Only a * in column 1 starts a comment line.
func TestStripABAPCommentColumnOne(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"* comment", ""},
		{"*\" IMPORTING", ""},
		{"    * s2.", "    * s2."},
		{"  rv = a * b. \" note", "  rv = a * b."},
		{"  lv = '\"'.", "  lv = '\"'."},
	} {
		if got := stripABAPComment(tc.in); got != tc.want {
			t.Errorf("stripABAPComment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The i64.load*_s/u cases of mem_ld_i64_ext each need a body; an empty WHEN
// before the shared one returns 0 for the signed loads.
func TestMemLdI64ExtNoEmptyWhen(t *testing.T) {
	src := compileClass(t, buildHelperModule(), "zcl_test_helpers")
	lines := strings.Split(src, "\n")
	for i := 0; i+1 < len(lines); i++ {
		cur, next := strings.TrimSpace(lines[i]), strings.TrimSpace(lines[i+1])
		if strings.HasPrefix(cur, "WHEN ") && strings.HasSuffix(cur, ".") && !strings.Contains(cur, ". ") &&
			(strings.HasPrefix(next, "WHEN ") || strings.HasPrefix(next, "ENDCASE.")) {
			t.Fatalf("empty WHEN at line %d: %q then %q", i+1, cur, next)
		}
	}
}

// The expected values the OSD cases get from wazero, pinned here so a change
// in the hand-built modules shows up in Go too.
func TestRuntimeSemanticsWazero(t *testing.T) {
	for _, m := range osdModules {
		var bin []byte
		var want []osdResult
		switch m.file {
		case "callind":
			bin = buildCallIndirectModule()
			want = []osdResult{{value: 11}, {value: 20}, {value: 7}, {value: 110}, {trap: true}, {trap: true}, {trap: true}, {trap: true}, {value: 14}}
		case "helpers":
			bin = buildHelperModule()
			want = []osdResult{{value: -1}, {value: 5}, {value: 255}, {value: -32768}, {value: 32767}, {value: 32768},
				{value: -2147483648}, {value: -1}, {value: 0}, {value: 0},
				{value: -3}, {value: -1}, {value: 0}, {value: -1}, {value: -4}, {value: 25},
				{value: -1}, {value: -3}, {value: 0}, {value: -1}, {value: -3}, {value: 1}}
		default:
			continue
		}
		got := wazeroResults(t, bin, m.cases)
		if len(got) != len(want) {
			t.Fatalf("%s: %d results, want %d", m.class, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s case %d %s%v = %+v, want %+v", m.class, i+1, m.cases[i].fn, m.cases[i].args, got[i], want[i])
			}
		}
	}
}

// shr_s32/shr_s64 divide by 2^k. In type i, 2^31 overflows (shift 31 dumps
// with CX_SY_ARITHMETIC_OVERFLOW on SAP), as 2^63 does in int8. The abaplint
// transpiler computes in JS numbers and does not overflow, so OSD cannot see
// this: check that the divisor is packed and no ipow( ) is left.
func TestShrSignedHelpersDivideInPacked(t *testing.T) {
	src := compileClass(t, buildHelperModule(), "zcl_test_helpers")
	for _, name := range []string{"shr_s32", "shr_s64"} {
		start := strings.Index(src, "METHOD "+name+".")
		if start < 0 {
			t.Fatalf("%s not emitted", name)
		}
		body := src[start:]
		body = body[:strings.Index(body, "ENDMETHOD.")]
		if strings.Contains(body, "ipow(") || !strings.Contains(body, "DATA lv_d TYPE p LENGTH 16 DECIMALS 0.") ||
			!strings.Contains(body, "lv_p = lv_p DIV lv_d.") {
			t.Errorf("%s does not divide in packed decimals:\n%s", name, body)
		}
	}
}
