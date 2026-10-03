package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func divisionPairs(is64 bool) [][2]int64 {
	min, max := int64(math.MinInt32), int64(math.MaxInt32)
	if is64 {
		min, max = math.MinInt64, math.MaxInt64
	}
	edges := []int64{0, 1, -1, 2, -2, 3, -3, 7, -7, min, max}
	var pairs [][2]int64
	for _, a := range edges {
		for _, b := range edges {
			pairs = append(pairs, [2]int64{a, b})
		}
	}
	pairs = append(pairs, [2]int64{35, 4}, [2]int64{-35, 4}, [2]int64{35, -4}, [2]int64{-35, -4})
	if is64 {
		// Unsigned halving/carry boundaries, odd negative dividends, and bits
		// beyond JavaScript's exact Number range must all remain observable.
		pairs = append(pairs, [][2]int64{
			{min + 1, 2}, {min + 1, 3}, {-1, max - 1}, {min, max - 1},
			{min, 1 << 62}, {-1, 1 << 62}, {min + 1, 1<<62 + 1}, {-1, 1<<62 + 1},
			{-1, 0xffffffff}, {0xffffffff, 3}, {1<<53 + 1, 3}, {-(1<<53 + 1), 3},
		}...)
	}
	rng := rand.New(rand.NewSource(20261003))
	for i := 0; i < 16; i++ {
		a, b := int64(rng.Uint64()), int64(rng.Uint64())
		if !is64 {
			a, b = int64(int32(a)), int64(int32(b))
		}
		pairs = append(pairs, [2]int64{a, b})
	}
	return pairs
}

// i32 exports take operands directly. i64 exports load all operand bits from
// an immutable data segment and expose both result halves through an i32 API.
// A handful of parameterized exports keeps the compiled class small even when
// the edge grid grows, unlike generating one function per constant pair.
func buildDivisionModule(is64 bool, pairs [][2]int64, helpers ...string) ([]byte, []osdCase) {
	w := newWasmBuilder()
	params := []ValType{ValI32, ValI32}
	if is64 {
		params = []ValType{ValI32}
	}
	w.addSection(1, buildTypeSection([]FuncType{{Params: params, Results: []ValType{ValI32}}}))
	var types []int
	var exports []Export
	var bodies [][]byte
	var cases []osdCase
	for _, h := range divisionHelperOps {
		if strings.HasSuffix(h.name, "64") != is64 || len(helpers) != 0 && !slices.Contains(helpers, h.name) {
			continue
		}
		halves := []string{""}
		if is64 {
			halves = []string{"_lo", "_hi"}
		}
		for _, half := range halves {
			code := []byte{OpLocalGet, 0, OpLocalGet, 1, h.op}
			if is64 {
				code = []byte{OpLocalGet, 0, OpI64Load, 3, 0, OpLocalGet, 0, OpI64Load, 3, 8, h.op}
				if half == "_hi" {
					code = append(code, OpI64Const, 32, OpI64ShrU)
				}
				code = append(code, OpI32WrapI64)
			}
			name := "probe_" + h.name + half
			exports = append(exports, Export{Name: name, Index: len(bodies)})
			types = append(types, 0)
			bodies = append(bodies, buildFuncBody(nil, code))
			for i, p := range pairs {
				args := []int32{int32(p[0]), int32(p[1])}
				if is64 {
					args = []int32{int32(i * 16)}
				}
				cases = append(cases, osdCase{fn: name, args: args})
			}
		}
	}
	w.addSection(3, buildFuncSection(types))
	if is64 {
		w.addSection(5, []byte{1, 0, 1})
	}
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	if is64 {
		var data []byte
		for _, p := range pairs {
			data = binary.LittleEndian.AppendUint64(data, uint64(p[0]))
			data = binary.LittleEndian.AppendUint64(data, uint64(p[1]))
		}
		segment := append([]byte{1, 0, OpI32Const, 0, OpEnd}, leb128u(uint32(len(data)))...)
		w.addSection(11, append(segment, data...))
	}
	return w.bytes(), cases
}

func divisionOSDCases(is64, remainder bool) []osdCase {
	_, cases := buildDivisionModule(is64, divisionPairs(is64))
	if !is64 {
		return cases
	}
	// Separate test classes keep each full i64 grid within the source budget.
	var selected []osdCase
	for _, c := range cases {
		if strings.Contains(c.fn, "rem_") == remainder {
			selected = append(selected, c)
		}
	}
	return selected
}

func TestDivisionFixtureWazero(t *testing.T) {
	for _, is64 := range []bool{false, true} {
		t.Run(fmt.Sprintf("i%d", map[bool]int{false: 32, true: 64}[is64]), func(t *testing.T) {
			pairs := divisionPairs(is64)
			bin, cases := buildDivisionModule(is64, pairs)
			got := wazeroResults(t, bin, cases)
			for i, c := range cases {
				pair := i % len(pairs)
				a, b := pairs[pair][0], pairs[pair][1]
				min := int64(math.MinInt32)
				if is64 {
					min = math.MinInt64
				}
				signed := strings.Contains(c.fn, "_s")
				remainder := strings.Contains(c.fn, "rem_")
				want := osdResult{trap: b == 0 || signed && !remainder && a == min && b == -1}
				if !want.trap {
					var value int64
					switch {
					case signed && remainder:
						value = a % b
					case signed:
						value = a / b
					case is64 && remainder:
						value = int64(uint64(a) % uint64(b))
					case is64:
						value = int64(uint64(a) / uint64(b))
					case remainder:
						value = int64(uint32(a) % uint32(b))
					default:
						value = int64(uint32(a) / uint32(b))
					}
					if strings.HasSuffix(c.fn, "_hi") {
						value >>= 32
					}
					want.value = int32(value)
				}
				if got[i] != want {
					t.Errorf("%s(%d, %d) = %+v, want %+v", c.fn, a, b, got[i], want)
				}
			}
		})
	}
}

var bareDivision = regexp.MustCompile(`(?:g=>)?s\d+(?:_i64)? = (?:g=>)?s\d+(?:_i64)? (?:/|DIV|MOD) (?:g=>)?s\d+(?:_i64)?\.`)

func TestDivisionGeneratedBackends(t *testing.T) {
	for _, h := range divisionHelperOps {
		t.Run(h.name, func(t *testing.T) {
			is64 := strings.HasSuffix(h.name, "64")
			bin, _ := buildDivisionModule(is64, [][2]int64{{-35, 4}}, h.name)
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
				t.Run(backend.String(), func(t *testing.T) {
					files := mustCompileWith(t, mod, "z_division", backend, 80).Files
					checkDivisionSource(t, files, h.name, backend != BackendClass)
				})
			}
			t.Run("multi-class", func(t *testing.T) {
				multi := mustCompileMultiClass(t, mod, "z_division", 80)
				checkDivisionSource(t, mustSplitFiles(t, multi, "z_division"), h.name, false)
				if !strings.Contains(multi.StateClass, "CLASS-METHODS "+h.name+" IMPORTING") {
					t.Errorf("missing state-class signature for %s", h.name)
				}
				called := false
				for _, src := range multi.ChunkClasses {
					called = called || strings.Contains(src, multi.StateName+"=>"+h.name+"( iv_a =")
				}
				if !called {
					t.Errorf("missing state-class call for %s", h.name)
				}
			})
		})
	}
}

func checkDivisionSource(t *testing.T, files map[string]string, helper string, fugr bool) {
	t.Helper()
	var all strings.Builder
	for name, src := range files {
		assertNoABAPComments(t, name, src)
		assertNestingBound(t, name, src)
		if parameterWrite.MatchString(src) {
			t.Errorf("%s writes an IMPORTING parameter", name)
		}
		if match := bareDivision.FindString(src); match != "" {
			t.Errorf("inline integer division/remainder: %s", match)
		}
		all.WriteString(src)
		all.WriteByte('\n')
	}
	checkLineLimit(t, files)
	src := all.String()
	if bad := kernelInvalidPatterns(src); len(bad) > 0 {
		t.Fatal(bad)
	}
	header, call := "METHOD "+helper+".", helper+"( iv_a ="
	if fugr {
		header, call = "FORM "+helper+" USING", "PERFORM "+helper+" USING"
	}
	definition := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(header))
	if len(definition.FindAllString(src, -1)) != 1 || !strings.Contains(src, call) {
		t.Fatalf("missing or duplicated %s helper/call", helper)
	}
	for _, h := range divisionHelperOps {
		if h.name != helper && strings.Contains(src, h.name) {
			t.Errorf("unused division helper %s emitted", h.name)
		}
	}
	for _, scope := range kernelScopeRE.FindAllStringSubmatch(src, -1) {
		if scope[1] != helper {
			continue
		}
		body := scope[3]
		if strings.Join(strings.Fields(body), " ") != strings.Join(strings.Fields(kernelRuntimeBody(helper)), " ") {
			t.Errorf("%s differs from the shared runtime body", helper)
		}
		for _, forbidden := range []string{" TYPE p", " / ", "DATA(", "abs("} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q", helper, forbidden)
			}
		}
		if importingI64Write.MatchString(body) {
			t.Errorf("%s writes an input parameter", helper)
		}
		guard := "IF iv_b = 0. " + wasmTrap + " ENDIF."
		if !strings.Contains(body, guard) {
			t.Errorf("%s does not explicitly raise the WASM trap on zero", helper)
		}
	}
}

func TestDivisionUnitTrapsUseWASMException(t *testing.T) {
	for _, is64 := range []bool{false, true} {
		pairs := divisionPairs(is64)
		bin, cases := buildDivisionModule(is64, pairs)
		want := wazeroResults(t, bin, cases)
		src := osdTestClassReplayTrap("zcl_division", cases, want, false, "cx_sy_dyn_call_illegal_method")
		checkTestClass(t, "zcl_division", src, want)
		if strings.Contains(src, "CATCH cx_root.") || !strings.Contains(src, "CATCH cx_sy_dyn_call_illegal_method.") {
			t.Fatal("division tests must catch only the WASM trap exception")
		}
	}
}
