package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
)

// buildSpecialFloatModule includes direct constants, constant reinterpret pairs,
// and runtime reinterpret inputs (nop, local, and computed bit patterns).
// Each exported function returns signed trunc_sat.
func buildSpecialFloatModule() ([]byte, []osdCase) {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Results: []ValType{ValI32}}}))
	var types []int
	var exports []Export
	var bodies [][]byte
	var cases []osdCase
	for _, bits := range []int{32, 64} {
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(math.NaN(), -1)} {
			for mode := 0; mode < 5; mode++ {
				var locals []ValType
				var code []byte
				if bits == 32 {
					pattern := math.Float32bits(float32(value))
					if mode != 0 {
						code = append([]byte{OpI32Const}, leb128s(int32(pattern))...)
						switch mode {
						case 2:
							code = append(code, OpNop)
						case 3:
							locals = []ValType{ValI32}
							code = append(code, OpLocalSet, 0, OpLocalGet, 0)
						case 4:
							code = append(code, OpI32Const, 0, OpI32Xor)
						}
						code = append(code, OpF32ReinterpretI32)
					} else {
						code = binary.LittleEndian.AppendUint32([]byte{OpF32Const}, pattern)
					}
					code = append(code, OpMiscPrefix, 0)
				} else {
					pattern := math.Float64bits(value)
					if mode != 0 {
						code = []byte{OpI64Const}
						v := int64(pattern)
						for {
							b := byte(v & 0x7f)
							v >>= 7
							if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
								code = append(code, b)
								break
							}
							code = append(code, b|0x80)
						}
						switch mode {
						case 2:
							code = append(code, OpNop)
						case 3:
							locals = []ValType{ValI64}
							code = append(code, OpLocalSet, 0, OpLocalGet, 0)
						case 4:
							code = append(code, OpI64Const, 0, OpI64Xor)
						}
						code = append(code, OpF64ReinterpretI64)
					} else {
						code = binary.LittleEndian.AppendUint64([]byte{OpF64Const}, pattern)
					}
					code = append(code, OpMiscPrefix, 2)
				}
				name := fmt.Sprintf("special%d", len(bodies))
				exports = append(exports, Export{Name: name, Index: len(bodies)})
				types = append(types, 0)
				bodies = append(bodies, buildFuncBody(locals, code))
				cases = append(cases, osdCase{fn: name})
			}
		}
	}
	w.addSection(3, buildFuncSection(types))
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	return w.bytes(), cases
}

func specialFloatOSDCases() []osdCase {
	_, cases := buildSpecialFloatModule()
	return cases
}

func TestSpecialFloatConstantsTrap(t *testing.T) {
	bin, cases := buildSpecialFloatModule()
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	redirects := DeduplicateFunctions(mod)
	check := func(t *testing.T, files map[string]string) {
		t.Helper()
		src := ""
		for _, file := range files {
			src += file + "\n"
		}
		flat := " " + strings.Join(strings.Fields(src), " ")
		for i, tc := range cases {
			name := tc.fn
			if canonical, ok := redirects[i]; ok {
				name = cases[canonical].fn
			}
			// Prefer the implementation FORM over the hybrid class wrapper.
			// The leading space avoids matching a PERFORM call.
			start := strings.Index(flat, " FORM "+name+" ")
			if start >= 0 {
				start++
			} else {
				start = strings.Index(flat, "METHOD "+name+".")
			}
			if start < 0 {
				match := regexp.MustCompile(`METHOD [a-zA-Z0-9_]+~` + regexp.QuoteMeta(name) + `\.`).FindStringIndex(flat)
				if match != nil {
					start = match[0]
				}
			}
			if start < 0 {
				t.Fatalf("missing function %s", tc.fn)
			}
			body := flat[start:]
			end := strings.Index(body, "ENDMETHOD.")
			if strings.HasPrefix(body, "FORM ") {
				end = strings.Index(body, "ENDFORM.")
			}
			body = body[:end]
			trap := strings.Index(body, wasmTrap)
			conversion := strings.Index(body, " <> ")
			runtimeInput := i%5 >= 2
			if runtimeInput {
				if !strings.Contains(body, "reinterpret_") || trap >= 0 {
					t.Fatalf("%s must use the runtime reinterpret check: %s", tc.fn, body)
				}
			} else if trap < 0 || conversion < trap || strings.Contains(body, "reinterpret_") ||
				strings.Contains(body, "'NaN'") || strings.Contains(body, "'Inf'") || strings.Contains(body, "'+Inf'") || strings.Contains(body, "'-Inf'") {
				t.Fatalf("%s must trap before trunc_sat: %s", tc.fn, body)
			}
		}
		checkLineLimit(t, files)
		for name, src := range files {
			assertNoABAPComments(t, name, src)
		}
	}
	for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
		t.Run(backend.String(), func(t *testing.T) {
			check(t, mustCompileWith(t, mod, "zcl_floattrap", backend, 80).Files)
		})
	}
	multi := mustCompileMultiClass(t, mod, "zcl_floattrap", 80)
	check(t, multi.ChunkClasses)

	// Pin the intentional difference from WASM: NaN trunc_sat returns zero
	// in wazero, whereas every ABAP case must trap while constructing the float.
	results := wazeroResults(t, bin, cases)
	for i, got := range results {
		want := []int32{0, math.MaxInt32, math.MinInt32, 0}[(i/5)%4]
		if got.trap || got.value != want {
			t.Fatalf("%s: WASM got %v, want %d", cases[i].fn, got, want)
		}
		results[i] = osdResult{trap: true}
	}
	tests := osdTestClass("zcl_floattrap", cases, results)
	checkTestClass(t, "zcl_floattrap", tests, results)
}

func TestTruncSatF32DoublePrecisionKnownGap(t *testing.T) {
	code := binary.LittleEndian.AppendUint32([]byte{OpF32Const}, math.Float32bits(2147483520))
	code = binary.LittleEndian.AppendUint32(append(code, OpF32Const), math.Float32bits(64))
	code = append(code, OpF32Add, OpMiscPrefix, 0)
	bin := buildSingleFuncWasm("gap", FuncType{Results: []ValType{ValI32}}, nil, code)
	got := wazeroResults(t, bin, []osdCase{{fn: "gap"}})[0]
	if got.trap || got.value != math.MaxInt32 {
		t.Fatalf("WASM f32 rounds to 2^31 and saturates: %v", got)
	}
	src := strings.Join(strings.Fields(compileClass(t, bin, "zcl_f32gap")), " ")
	start := strings.Index(src, "METHOD gap.")
	body := src[start:]
	body = body[strings.Index(body, "s0_f = "):strings.Index(body, "ENDMETHOD.")]
	// Pin the current double-precision emission, including the unrounded
	// producer and the saturation bounds. A future f32 fix must update this.
	want := "s0_f = '2147483520.000000'. s1_f = '64.000000'. s0_f = s0_f + s1_f. " +
		"IF s0_f <> s0_f. s0 = 0. ELSEIF s0_f <= '-2147483648'. s0 = -2147483648. " +
		"ELSEIF s0_f >= '2147483648'. s0 = 2147483647. ELSE. s0 = trunc( s0_f ). ENDIF. rv = s0. "
	if body != want {
		t.Fatalf("known f32 gap changed; review precision behaviour:\n%s", body)
	}
	abapResult := int32(math.Trunc(float64(2147483520) + float64(64)))
	if abapResult != 2147483584 || abapResult == got.value {
		t.Fatalf("expected existing double-precision gap: ABAP=%d WASM=%d", abapResult, got.value)
	}
}

func TestSpecialFloatTrapStaysOnPath(t *testing.T) {
	for _, op := range []byte{OpF32Const, OpF64Const} {
		code := []byte{OpLocalGet, 0, OpIf, byte(ValI32), op}
		if op == OpF32Const {
			code = binary.LittleEndian.AppendUint32(code, math.Float32bits(float32(math.NaN())))
		} else {
			code = binary.LittleEndian.AppendUint64(code, math.Float64bits(math.NaN()))
		}
		code = append(code, OpDrop, OpI32Const, 1, OpElse, OpI32Const, 7, OpEnd)
		bin := buildSingleFuncWasm("conditional", FuncType{Params: []ValType{ValI32}, Results: []ValType{ValI32}}, nil, code)
		src := strings.Join(strings.Fields(compileClass(t, bin, "zcl_conditional")), " ")
		body := src[strings.Index(src, "METHOD conditional."):]
		body = body[:strings.Index(body, "ENDMETHOD.")]
		branch, trap, otherwise := strings.Index(body, "IF "), strings.Index(body, wasmTrap), strings.Index(body, "ELSE.")
		if branch < 0 || trap < branch || otherwise < trap || strings.Count(body, wasmTrap) != 1 {
			t.Fatalf("constant must trap only inside its branch, even when dropped: %s", body)
		}
	}
}

func TestRuntimeReinterpretExponentCheck(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    ValType
		output   ValType
		op       byte
		exponent string
	}{
		{"reinterpret_i32_f32", ValI32, ValF32, OpF32ReinterpretI32, "255"},
		{"reinterpret_i64_f64", ValI64, ValF64, OpF64ReinterpretI64, "2047"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := buildSingleFuncWasm("reinterpret", FuncType{Params: []ValType{tc.input}, Results: []ValType{tc.output}}, nil,
				[]byte{OpLocalGet, 0, tc.op})
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			multi := mustCompileMultiClass(t, mod, "zcl_reinterpret", 80)
			for _, src := range []string{compileClass(t, bin, "zcl_reinterpret"), multi.StateClass} {
				flat := strings.Join(strings.Fields(src), " ")
				start := strings.Index(flat, "METHOD "+tc.name+".")
				if start < 0 {
					t.Fatal("missing runtime helper")
				}
				body := flat[start:]
				body = body[:strings.Index(body, "ENDMETHOD.")]
				guard := "IF lv_exp = " + tc.exponent + ". " + wasmTrap + " ENDIF."
				guardAt := strings.Index(body, guard)
				decodeAt := strings.Index(body, "rv = lv_frac.")
				if guardAt < 0 || decodeAt <= guardAt || parameterWrite.MatchString(body) {
					t.Fatalf("helper must check exponent and trap before decoding: %s", body)
				}
			}
		})
	}
}
