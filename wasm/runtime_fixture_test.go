package wasm

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"
)

// Each export has no arguments and returns an observable i32 result. i64
// results have separate low/high exports so wrap cannot hide a broken high half.
func runtimeFixture() ([]byte, []osdCase) {
	var exports []Export
	var bodies [][]byte
	var cases []osdCase
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	i32 := func(v int64) []byte { return append([]byte{OpI32Const}, leb128s(int32(v))...) }
	i64 := func(v int64) []byte { return append([]byte{OpI64Const}, leb128s64(v)...) }
	f32 := func(v float32) []byte {
		return binary.LittleEndian.AppendUint32([]byte{OpF32Const}, math.Float32bits(v))
	}
	f64 := func(v float64) []byte {
		return binary.LittleEndian.AppendUint64([]byte{OpF64Const}, math.Float64bits(v))
	}
	add := func(name string, code []byte) {
		name = "probe_" + name
		exports = append(exports, Export{Name: name, Kind: 0, Index: len(bodies)})
		bodies = append(bodies, buildFuncBody(nil, code))
		cases = append(cases, osdCase{fn: name})
	}
	add64 := func(name string, code []byte) {
		add(name+"_lo", cat(code, []byte{OpI32WrapI64}))
		add(name+"_hi", cat(code, i64(32), []byte{OpI64ShrU, OpI32WrapI64}))
	}
	for _, spec := range []struct {
		name       string
		op32, op64 byte
	}{
		{"clz", OpI32Clz, OpI64Clz}, {"ctz", OpI32Ctz, OpI64Ctz}, {"popcnt", OpI32Popcnt, OpI64Popcnt},
	} {
		for j, v := range []int64{0, 1, 16, -1, -2147483648} {
			add(fmt.Sprintf("%s32_%d", spec.name, j), cat(i32(v), []byte{spec.op32}))
		}
		for j, v := range []int64{0, 1, 16, -1, math.MinInt64, 1 << 62} {
			add(fmt.Sprintf("%s64_%d", spec.name, j), cat(i64(v), []byte{spec.op64, OpI32WrapI64}))
		}
	}
	for _, spec := range []struct {
		name       string
		op32, op64 byte
	}{
		{"and", OpI32And, OpI64And}, {"or", OpI32Or, OpI64Or}, {"xor", OpI32Xor, OpI64Xor},
		{"shl", OpI32Shl, OpI64Shl}, {"shrs", OpI32ShrS, OpI64ShrS}, {"shru", OpI32ShrU, OpI64ShrU},
		{"rotl", OpI32Rotl, OpI64Rotl}, {"rotr", OpI32Rotr, OpI64Rotr},
	} {
		for j, k := range []int64{0, 1, 31, 32, 63, 65} {
			add(fmt.Sprintf("%s32_%d", spec.name, j), cat(i32(-2147483647), i32(k), []byte{spec.op32}))
			add64(fmt.Sprintf("%s64_%d", spec.name, j), cat(i64(math.MinInt64+1), i64(k), []byte{spec.op64}))
		}
	}
	for _, spec := range []struct {
		name       string
		op32, op64 byte
		compare    bool
	}{
		{"divu", OpI32DivU, OpI64DivU, false}, {"remu", OpI32RemU, OpI64RemU, false},
		{"ltu", OpI32LtU, OpI64LtU, true}, {"gtu", OpI32GtU, OpI64GtU, true},
		{"leu", OpI32LeU, OpI64LeU, true}, {"geu", OpI32GeU, OpI64GeU, true},
	} {
		for j, p := range [][2]int64{{-1, 1}, {-1, 3}, {1, -1}, {-2, -1}, {-1, -1}, {7, 3}} {
			add(fmt.Sprintf("%s32_%d", spec.name, j), cat(i32(p[0]), i32(p[1]), []byte{spec.op32}))
			code := cat(i64(p[0]), i64(p[1]), []byte{spec.op64})
			if spec.compare {
				add(fmt.Sprintf("%s64_%d", spec.name, j), code)
			} else {
				add64(fmt.Sprintf("%s64_%d", spec.name, j), code)
			}
		}
	}
	for _, spec := range []struct {
		name string
		op   byte
	}{{"add64", OpI64Add}, {"sub64", OpI64Sub}, {"mul64", OpI64Mul}} {
		add64(spec.name, cat(i64(math.MaxInt64), i64(3), []byte{spec.op}))
	}
	for j, p := range [][2]int64{{math.MinInt64, 2}, {math.MinInt64, math.MaxInt64}, {-1, math.MaxInt64}, {math.MinInt64, math.MinInt64}, {math.MaxInt64, math.MinInt64}} {
		add64(fmt.Sprintf("divedge%d", j), cat(i64(p[0]), i64(p[1]), []byte{OpI64DivU}))
		add64(fmt.Sprintf("remedge%d", j), cat(i64(p[0]), i64(p[1]), []byte{OpI64RemU}))
	}
	add64("extend_u32", cat(i32(-1), []byte{OpI64ExtendI32U}))
	add64("extend_s32", cat(i32(-1), []byte{OpI64ExtendI32S}))
	for _, spec := range []struct {
		name string
		op   byte
		wide bool
	}{
		{"extend8_32", OpI32Extend8S, false}, {"extend16_32", OpI32Extend16S, false},
		{"extend8_64", OpI64Extend8S, true}, {"extend16_64", OpI64Extend16S, true}, {"extend32_64", OpI64Extend32S, true},
	} {
		if spec.wide {
			add64(spec.name, cat(i64(4294967295), []byte{spec.op}))
		} else {
			add(spec.name, cat(i32(65535), []byte{spec.op}))
		}
	}
	add("convert_u64", cat(i64(-1), []byte{OpF64ConvertI64U}, f64(8589934592), []byte{OpF64Div, OpI32TruncF64U}))
	add("trunc_u32", cat(f64(4294967295), []byte{OpI32TruncF64U}))
	add64("trunc_u64", cat(f64(13835058055282163712), []byte{OpI64TruncF64U}))
	add("copysign", cat(f64(13.25), f64(-1), []byte{OpF64Copysign}, f64(4), []byte{OpF64Mul, OpI32TruncF64S}))
	add("reinterpret32", cat(f32(-13.25), []byte{OpI32ReinterpretF32}))
	add("reinterpret32_back", cat(i32(int64(int32(math.Float32bits(-13.25)))), []byte{OpF32ReinterpretI32}, f32(4), []byte{OpF32Mul, OpI32TruncF32S}))
	add64("reinterpret64", cat(f64(-13.25), []byte{OpI64ReinterpretF64}))
	add("reinterpret64_back", cat(i64(int64(math.Float64bits(-13.25))), []byte{OpF64ReinterpretI64}, f64(4), []byte{OpF64Mul, OpI32TruncF64S}))
	// Both overlap directions, including all bytes in the copied region.
	for j, p := range [][2]int64{{65, 64}, {64, 65}} {
		code := cat(i32(64), i64(0x0807060504030201), []byte{OpI64Store, 0, 0}, i32(p[0]), i32(p[1]), i32(7), []byte{0xfc, 10, 0, 0}, i32(64), []byte{OpI64Load, 0, 0})
		add64(fmt.Sprintf("copy%d", j), code)
	}
	add64("fill", cat(i32(64), i32(511), i32(8), []byte{0xfc, 11, 0}, i32(64), []byte{OpI64Load, 0, 0}))
	loadInit := cat(i32(0), i32(-2147450625), []byte{OpI32Store, 0, 0})
	add("load16s", cat(loadInit, i32(0), []byte{OpI32Load16S, 0, 0}))
	for j, op := range []byte{OpI64Load8S, OpI64Load8U, OpI64Load16S, OpI64Load16U, OpI64Load32S, OpI64Load32U} {
		add64(fmt.Sprintf("loadext%d", j), cat(loadInit, i32(0), []byte{op, 0, 0}))
	}
	for j, op := range []byte{OpI64Store8, OpI64Store16, OpI64Store32} {
		add64(fmt.Sprintf("storetrunc%d", j), cat(i32(64), i64(0), []byte{OpI64Store, 0, 0}, i32(64), i64(-1), []byte{op, 0, 0}, i32(64), []byte{OpI64Load, 0, 0}))
	}
	add("loadf32", cat(i32(8), f32(-13.25), []byte{OpF32Store, 0, 0}, i32(8), []byte{OpF32Load, 0, 0}, f32(4), []byte{OpF32Mul, OpI32TruncF32S}))
	add("loadf64", cat(i32(16), f64(-13.25), []byte{OpF64Store, 0, 0}, i32(16), []byte{OpF64Load, 0, 0}, f64(4), []byte{OpF64Mul, OpI32TruncF64S}))
	add("storef32_bits", cat(i32(64), f32(-13.25), []byte{OpF32Store, 0, 0}, i32(64), []byte{OpI32Load, 0, 0}))
	add64("storef64_bits", cat(i32(64), f64(-13.25), []byte{OpF64Store, 0, 0}, i32(64), []byte{OpI64Load, 0, 0}))
	add("roundtripf32", cat(i32(64), f32(1.5), []byte{OpF32Store, 0, 0}, i32(64), []byte{OpF32Load, 0, 0}, f32(4), []byte{OpF32Mul, OpI32TruncF32S}))
	add("roundtripf64", cat(i32(64), f64(1.5), []byte{OpF64Store, 0, 0}, i32(64), []byte{OpF64Load, 0, 0}, f64(4), []byte{OpF64Mul, OpI32TruncF64S}))
	for _, zero := range []struct {
		name  string
		value float64
	}{{"zero", 0}, {"negative_zero", math.Copysign(0, -1)}} {
		add("storef32_"+zero.name, cat(i32(64), f32(13.25), []byte{OpF32Store, 0, 0}, i32(64), f32(float32(zero.value)), []byte{OpF32Store, 0, 0}, i32(64), []byte{OpI32Load, 0, 0}))
		add64("storef64_"+zero.name, cat(i32(64), f64(13.25), []byte{OpF64Store, 0, 0}, i32(64), f64(zero.value), []byte{OpF64Store, 0, 0}, i32(64), []byte{OpI64Load, 0, 0}))
	}
	add("reinterpret32_negative_zero", cat(i32(math.MinInt32), []byte{OpF32ReinterpretI32, OpI32ReinterpretF32}))
	add64("reinterpret64_negative_zero", cat(i64(math.MinInt64), []byte{OpF64ReinterpretI64, OpI64ReinterpretF64}))
	for j, bits := range []uint32{0x7f800000, 0xff800000, 0x7fc00000, 0xffc00000, 0x7f800001, 0xff800001} {
		// Truncation also traps in wazero, giving an independent trap oracle
		// while ABAP must trap earlier, at the load/reinterpret operation.
		add(fmt.Sprintf("reinterpret32_nonfinite%d", j), cat(i32(int64(int32(bits))), []byte{OpF32ReinterpretI32, OpI32TruncF32S}))
		add(fmt.Sprintf("loadf32_nonfinite%d", j), cat(i32(64), i32(int64(int32(bits))), []byte{OpI32Store, 0, 0}, i32(64), []byte{OpF32Load, 0, 0, OpI32TruncF32S}))
	}
	for j, bits := range []uint64{0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000000, 0xfff8000000000000, 0x7ff0000000000001, 0xfff0000000000001} {
		add(fmt.Sprintf("reinterpret64_nonfinite%d", j), cat(i64(int64(bits)), []byte{OpF64ReinterpretI64, OpI32TruncF64S}))
		add(fmt.Sprintf("loadf64_nonfinite%d", j), cat(i32(64), i64(int64(bits)), []byte{OpI64Store, 0, 0}, i32(64), []byte{OpF64Load, 0, 0, OpI32TruncF64S}))
	}
	for _, n := range []int64{0, 1, 3, 7, 9, 257, 65536} {
		// Sentinels check both fill boundaries, including the empty range.
		start := int64(0)
		if n < 65536 {
			start = 64
		}
		code := cat(i32(start), i32(0x55), []byte{OpI32Store8, 0, 0}, i32(start+n-1), i32(0x55), []byte{OpI32Store8, 0, 0})
		if n == 0 {
			code = cat(i32(start), i32(0x55), []byte{OpI32Store8, 0, 0})
		}
		if start > 0 {
			code = cat(code, i32(start-1), i32(0x55), []byte{OpI32Store8, 0, 0}, i32(start+n), i32(0x55), []byte{OpI32Store8, 0, 0})
		}
		code = cat(code, i32(start), i32(0x1ab), i32(n), []byte{0xfc, 11, 0})
		add(fmt.Sprintf("fill%d_first", n), cat(code, i32(start), []byte{OpI32Load8U, 0, 0}))
		if n > 0 {
			add(fmt.Sprintf("fill%d_last", n), cat(code, i32(start+n-1), []byte{OpI32Load8U, 0, 0}))
			add(fmt.Sprintf("fill%d_middle", n), cat(code, i32(start+n/2), []byte{OpI32Load8U, 0, 0}))
		}
		if start > 0 {
			add(fmt.Sprintf("fill%d_before", n), cat(code, i32(start-1), []byte{OpI32Load8U, 0, 0}))
			add(fmt.Sprintf("fill%d_after", n), cat(code, i32(start+n), []byte{OpI32Load8U, 0, 0}))
		}
	}
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Results: []ValType{ValI32}}}))
	w.addSection(3, buildFuncSection(make([]int, len(bodies))))
	w.addSection(5, []byte{1, 0, 1})
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	data := make([]byte, 24)
	binary.LittleEndian.PutUint32(data, 0x800080ff)
	binary.LittleEndian.PutUint32(data[8:], math.Float32bits(-13.25))
	binary.LittleEndian.PutUint64(data[16:], math.Float64bits(-13.25))
	segment := append([]byte{1, 0, OpI32Const, 0, OpEnd}, leb128u(uint32(len(data)))...)
	w.addSection(11, append(segment, data...))
	return w.bytes(), cases
}

func TestRuntimeFixtureCoversEveryHelper(t *testing.T) {
	bin, cases := runtimeFixture()
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	src := runtimeFixtureClass(mod, "zcl_abapiti_runtime_helpers")
	declarations, _ := runtimeTemplates()
	for name := range declarations {
		if !strings.Contains(src, "METHOD "+name+".") || !strings.Contains(src, name+"(") {
			t.Errorf("fixture does not exercise %s", name)
		}
	}
	for i, result := range wazeroResults(t, bin, cases) {
		if wantTrap := strings.Contains(cases[i].fn, "_nonfinite"); result.trap != wantTrap {
			t.Errorf("%s trap = %v, want %v", cases[i].fn, result.trap, wantTrap)
		}
	}
	checkLineLimit(t, map[string]string{"fixture": src})
	if bad := kernelInvalidPatterns(src); len(bad) > 0 {
		t.Fatal(bad)
	}
}

// Allocation and active data initialization are normally emitted inline. Run
// their shared runtime counterparts too, with exactly the module's memory image.
func runtimeFixtureClass(mod *Module, class string) string {
	src, err := Compile(mod, class)
	if err != nil {
		panic(err)
	}
	declarations, bodies := singleClassRuntimeTemplates()
	var defs, impl strings.Builder
	for _, name := range []string{"alloc_mem", "mem_init"} {
		defs.WriteString(declarations[name] + "\n")
		impl.WriteString("METHOD " + name + ".\n" + bodies[name] + "\nENDMETHOD.\n")
	}
	src = strings.Replace(src, "PRIVATE SECTION.", "PRIVATE SECTION.\n"+defs.String(), 1)
	src = strings.Replace(src, "CLASS "+class+" IMPLEMENTATION.", "CLASS "+class+" IMPLEMENTATION.\n"+impl.String(), 1)
	// The constructor still uses the normal allocator; this additional allocation
	// tests the legacy helper's chunk and remainder paths, then trims to one page.
	init := "mv_mem = alloc_mem( 65537 ).\n"
	for _, seg := range mod.Data {
		init += fmt.Sprintf("mem_init( iv_off = %d iv_hex = '%s' ).\n", seg.Offset, strings.ToUpper(hex.EncodeToString(seg.Data)))
	}
	init += fmt.Sprintf("mv_mem = mv_mem+0(%d).\n", mod.Memory.Min*65536)
	start := strings.Index(src, "METHOD constructor.")
	end := start + strings.Index(src[start:], "ENDMETHOD.")
	src = src[:end] + init + src[end:]
	return wrapLongLines(src)
}

// ABAP has no portable 7.02/JS sign-bit access for TYPE f zero. Keep the
// independent IEEE expectations, then explicitly apply only this known gap.
func runtimeFixtureResults(t *testing.T, bin []byte, cases []osdCase) []osdResult {
	t.Helper()
	want := wazeroResults(t, bin, cases)
	for i, c := range cases {
		if strings.Contains(c.fn, "negative_zero") {
			ieee := int32(math.MinInt32)
			if strings.HasSuffix(c.fn, "_lo") {
				ieee = 0
			}
			if want[i].trap || want[i].value != ieee {
				t.Fatalf("%s: wazero negative zero = %+v, want %d", c.fn, want[i], ieee)
			}
			want[i].value = 0
		}
	}
	return want
}

func TestRuntimeFixtureKnownNegativeZeroGap(t *testing.T) {
	bin, cases := runtimeFixture()
	want := runtimeFixtureResults(t, bin, cases)
	for i, c := range cases {
		if strings.Contains(c.fn, "negative_zero") && (want[i].trap || want[i].value != 0) {
			t.Errorf("%s: ABAP zero normalization = %+v", c.fn, want[i])
		}
	}
}

func TestRuntimeFixtureIndependentCalls(t *testing.T) {
	bin, cases := runtimeFixture()
	want := wazeroResults(t, bin, cases)
	// Reverse order exposes reads of memory changed by later fill/store probes.
	reversed := append([]osdCase(nil), cases...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	got := wazeroResults(t, bin, reversed)
	for i, c := range cases {
		if fresh := wazeroResults(t, bin, []osdCase{c})[0]; fresh != want[i] {
			t.Errorf("%s fresh = %+v, sequential = %+v", c.fn, fresh, want[i])
		}
		if got[len(cases)-1-i] != want[i] {
			t.Errorf("%s reversed = %+v, sequential = %+v", c.fn, got[len(cases)-1-i], want[i])
		}
	}
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	names := moduleFunctionNames(mod)
	src := runtimeFixtureClass(mod, "zcl_abapiti_runtime_helpers")
	checkNamedMethods(t, src, names, true)
	tests := osdTestClassReplay("zcl_abapiti_runtime_helpers", cases, runtimeFixtureResults(t, bin, cases), false)
	if calls := strings.Count(tests, "lv_act = lo->"); calls != len(cases) {
		t.Fatalf("independent test calls = %d, want %d", calls, len(cases))
	}
	for _, name := range names {
		if strings.Count(tests, "lo->"+name+"(") != 1 {
			t.Errorf("test must call %s exactly once", name)
		}
	}
	checkLineLimit(t, map[string]string{"test class": tests})
}

func TestOSDTestClassReplay(t *testing.T) {
	cases := []osdCase{{fn: "grow"}, {fn: "size"}, {fn: "grow"}}
	want := []osdResult{{trap: true}, {value: 1}, {value: 2}}
	for _, replay := range []bool{false, true} {
		src := osdTestClassReplay("zcl_stateful", cases, want, replay)
		expected := len(cases)
		if replay {
			expected = len(cases) * (len(cases) + 1) / 2
		}
		if calls := strings.Count(src, "lv_act = lo->"); calls != expected {
			t.Errorf("replay %v: calls %d, want %d", replay, calls, expected)
		}
		checkTestClass(t, "zcl_stateful", src, want)
	}
}
