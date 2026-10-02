package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

var truncSatInputs = []float64{
	0, math.Copysign(0, -1), -0.5, -1, 1.9, -1.9,
	math.Nextafter(1, 0), math.Nextafter(-1, 0),
	2147483647, 2147483648, -2147483648, -2147483649,
	4294967295, 4294967296, 4294967295.75,
	9223372036854775808, -9223372036854775808,
	math.Nextafter(9223372036854775808, 0),
	math.Nextafter(-9223372036854775808, math.Inf(-1)),
	1.8446744073709552e19, math.Nextafter(1.8446744073709552e19, 0),
	math.MaxFloat32, -math.MaxFloat32,
}

func buildTruncSatModule(special bool) ([]byte, []osdCase, []int32) {
	inputs := append([]float64(nil), truncSatInputs...)
	if special {
		inputs = append(inputs, math.NaN(), math.Inf(1), math.Inf(-1))
	}
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Results: []ValType{ValI32}}}))
	var types []int
	var exports []Export
	var bodies [][]byte
	var cases []osdCase
	var expected []int32
	for op := byte(0); op < 8; op++ {
		for i, input := range inputs {
			value := input
			var code []byte
			if op&2 == 0 {
				value = float64(float32(input))
				code = binary.LittleEndian.AppendUint32([]byte{OpF32Const}, math.Float32bits(float32(input)))
			} else {
				code = binary.LittleEndian.AppendUint64([]byte{OpF64Const}, math.Float64bits(input))
			}
			code = append(code, OpMiscPrefix, op)
			bits := truncSatExpected(op, value)
			halves := 1
			if op >= 4 {
				halves = 2
			}
			for half := 0; half < halves; half++ {
				body := append([]byte(nil), code...)
				if op >= 4 {
					if half == 1 {
						body = append(body, OpI64Const, 32, OpI64ShrS)
					}
					body = append(body, OpI32WrapI64)
				}
				name := fmt.Sprintf("sat%d_v%d_h%d", op, i, half)
				exports = append(exports, Export{Name: name, Index: len(bodies)})
				types = append(types, 0)
				bodies = append(bodies, buildFuncBody(nil, body))
				cases = append(cases, osdCase{fn: name})
				expected = append(expected, int32(bits>>uint(32*half)))
			}
		}
	}
	w.addSection(3, buildFuncSection(types))
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	return w.bytes(), cases, expected
}

func truncSatExpected(op byte, value float64) uint64 {
	if math.IsNaN(value) {
		return 0
	}
	width := 32
	if op >= 4 {
		width = 64
	}
	if op&1 != 0 {
		if value <= 0 {
			return 0
		}
		if value >= math.Ldexp(1, width) {
			if width == 32 {
				return math.MaxUint32
			}
			return math.MaxUint64
		}
		return uint64(math.Trunc(value))
	}
	bound := math.Ldexp(1, width-1)
	if value <= -bound {
		if width == 32 {
			return uint64(1 << 31)
		}
		return uint64(1 << 63)
	}
	if value >= bound {
		if width == 32 {
			return math.MaxInt32
		}
		return math.MaxInt64
	}
	return uint64(int64(math.Trunc(value)))
}

func truncSatOSDCases() []osdCase {
	_, cases, _ := buildTruncSatModule(false)
	return cases
}

func TestTruncSatWazero(t *testing.T) {
	for _, special := range []bool{false, true} {
		bin, cases, expected := buildTruncSatModule(special)
		results := wazeroResults(t, bin, cases)
		for i, result := range results {
			if result.trap || result.value != expected[i] {
				t.Fatalf("%s: wazero=%v expected=%d", cases[i].fn, result, expected[i])
			}
		}
		if !special {
			src := compileClass(t, bin, "zcl_trunc_sat")
			checkLineLimit(t, map[string]string{"sat": src})
			assertNoABAPComments(t, "sat", src)
			if strings.Contains(src, "zcl_wasm_rt") || strings.Contains(src, "TYPE p") || strings.Contains(src, " / ") {
				t.Fatal("trunc_sat class must be self-contained without packed arithmetic or division")
			}
		}
	}
}

func TestTruncSatParameterTypes(t *testing.T) {
	for op := byte(0); op < 8; op++ {
		input, output := ValF32, ValI32
		if op&2 != 0 {
			input = ValF64
		}
		if op >= 4 {
			output = ValI64
		}
		bin := buildSingleFuncWasm("sat", FuncType{Params: []ValType{input}, Results: []ValType{output}}, nil,
			[]byte{OpLocalGet, 0, OpMiscPrefix, op})
		src := compileClass(t, bin, "zcl_sat")
		if parameterWrite.MatchString(src) || !strings.Contains(src, "p0 TYPE f") ||
			!strings.Contains(src, "RETURNING VALUE(rv) TYPE "+output.ABAPType()) {
			t.Fatalf("opcode %d: incorrect parameter/result types or write to IMPORTING parameter", op)
		}
		if op >= 4 && !strings.Contains(src, "s0_i64") {
			t.Fatalf("opcode %d must use an int8 stack slot", op)
		}
	}
}

func TestUnsupportedOpcodes(t *testing.T) {
	for _, code := range [][]byte{
		{0xff}, {OpMiscPrefix, 8, 0, 0}, {OpMiscPrefix, 0x80, 2},
		{OpSIMDPrefix, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{OpSIMDPrefix, 0x80, 2}, {OpAtomicPrefix, 0x10, 0, 0},
	} {
		mod, err := Parse(buildSingleFuncWasm("bad", FuncType{}, nil, code))
		if err != nil {
			t.Fatal(err)
		}
		mod.Functions[0].Index = 37
		want := fmt.Sprintf("unsupported opcode 0x%02X", code[0])
		if code[0] >= OpMiscPrefix && code[0] <= OpAtomicPrefix {
			want += fmt.Sprintf(" 0x%02X", mod.Functions[0].Code[0].MiscOp)
		}
		want += " in function 37"
		src, err := Compile(mod, "zcl_bad")
		if src != "" || err == nil || err.Error() != want {
			t.Fatalf("Compile: src=%q err=%v want=%s", src, err, want)
		}
		for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
			out, err := CompileWith(mod, "zbad", backend, 1)
			if out != nil || err == nil || err.Error() != want {
				t.Fatalf("%s: err=%v want=%s", backend, err, want)
			}
		}
		out, err := CompileMultiClass(mod, "zcl_bad", 1)
		if out != nil || err == nil || err.Error() != want {
			t.Fatalf("multi-class: err=%v want=%s", err, want)
		}
	}
}
