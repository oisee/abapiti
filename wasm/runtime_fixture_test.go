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
	add("load16s", cat(i32(0), []byte{OpI32Load16S, 0, 0}))
	for j, op := range []byte{OpI64Load8S, OpI64Load8U, OpI64Load16S, OpI64Load16U, OpI64Load32S, OpI64Load32U} {
		add64(fmt.Sprintf("loadext%d", j), cat(i32(0), []byte{op, 0, 0}))
	}
	for j, op := range []byte{OpI64Store8, OpI64Store16, OpI64Store32} {
		add64(fmt.Sprintf("storetrunc%d", j), cat(i32(64), i64(-1), []byte{op, 0, 0}, i32(64), []byte{OpI64Load, 0, 0}))
	}
	add("loadf32", cat(i32(8), []byte{OpF32Load, 0, 0}, f32(4), []byte{OpF32Mul, OpI32TruncF32S}))
	add("loadf64", cat(i32(16), []byte{OpF64Load, 0, 0}, f64(4), []byte{OpF64Mul, OpI32TruncF64S}))
	add("storef32_bits", cat(i32(64), f32(-13.25), []byte{OpF32Store, 0, 0}, i32(64), []byte{OpI32Load, 0, 0}))
	add64("storef64_bits", cat(i32(64), f64(-13.25), []byte{OpF64Store, 0, 0}, i32(64), []byte{OpI64Load, 0, 0}))
	add("roundtripf32", cat(i32(64), f32(1.5), []byte{OpF32Store, 0, 0}, i32(64), []byte{OpF32Load, 0, 0}, f32(4), []byte{OpF32Mul, OpI32TruncF32S}))
	add("roundtripf64", cat(i32(64), f64(1.5), []byte{OpF64Store, 0, 0}, i32(64), []byte{OpF64Load, 0, 0}, f64(4), []byte{OpF64Mul, OpI32TruncF64S}))
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
		if result.trap {
			t.Errorf("%s unexpectedly traps", cases[i].fn)
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
	declarations, bodies := runtimeTemplates()
	var defs, impl strings.Builder
	for _, name := range []string{"alloc_mem", "mem_init"} {
		defs.WriteString(declarations[name] + "\n")
		impl.WriteString("METHOD " + name + ".\n" + bodies[name] + "\nENDMETHOD.\n")
	}
	src = strings.Replace(src, "PRIVATE SECTION.", "PRIVATE SECTION.\n"+defs.String(), 1)
	src = strings.Replace(src, "CLASS "+class+" IMPLEMENTATION.", "CLASS "+class+" IMPLEMENTATION.\n"+impl.String(), 1)
	// The constructor still uses the normal allocator; this additional allocation
	// tests the legacy helper's chunk and remainder paths, then copies the image.
	init := "DATA lv_probe TYPE xstring.\nlv_probe = alloc_mem( 65537 ).\n"
	for _, seg := range mod.Data {
		init += fmt.Sprintf("mem_init( EXPORTING iv_off = %d iv_hex = '%s' CHANGING cv_mem = lv_probe ).\n", seg.Offset, strings.ToUpper(hex.EncodeToString(seg.Data)))
	}
	init += "REPLACE SECTION OFFSET 0 LENGTH 24 OF mv_mem WITH lv_probe+0(24) IN BYTE MODE.\n"
	start := strings.Index(src, "METHOD constructor.")
	end := start + strings.Index(src[start:], "ENDMETHOD.")
	src = src[:end] + init + src[end:]
	return wrapLongLines(src)
}
