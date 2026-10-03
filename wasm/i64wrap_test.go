package wasm

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

type i64WrapCase struct {
	name string
	op   byte
	a, b int64
}

// Constants keep the ABAP Unit public API i32-only while exercising all 64
// operand bits. Each result is observed in two exports, so high-bit errors
// cannot hide behind i32.wrap_i64.
var i64WrapCases = func() []i64WrapCase {
	cases := []i64WrapCase{
		{"max_add", OpI64Add, math.MaxInt64, 1},
		{"min_sub", OpI64Sub, math.MinInt64, 1},
		{"min_mul", OpI64Mul, math.MinInt64, -1},
		{"u32_mul", OpI64Mul, 0xffffffff, 0xffffffff},
		{"fnv_mul", OpI64Mul, int64(-3750763034362895579), 1099511628211},
		{"neg_mul", OpI64Mul, -1, -1},
		{"min_add", OpI64Add, math.MinInt64, -1},
		{"max_sub", OpI64Sub, math.MaxInt64, -1},
		{"borrow_sub", OpI64Sub, 0, 1},
		{"carry_add", OpI64Add, 0xffffffff, 1},
		{"min_zero", OpI64Mul, math.MinInt64, 0},
		{"max_mul", OpI64Mul, math.MaxInt64, math.MaxInt64},
		{"min_min", OpI64Mul, math.MinInt64, math.MinInt64},
	}
	rng := rand.New(rand.NewSource(64))
	for i := 0; i < 8; i++ {
		a, b := int64(rng.Uint64()), int64(rng.Uint64())
		for _, h := range i64HelperOps {
			cases = append(cases, i64WrapCase{fmt.Sprintf("%s_r%d", h.name, i), h.op, a, b})
		}
	}
	return cases
}()

func i64WrapOSDCases() []osdCase {
	var cases []osdCase
	for _, half := range []string{"lo", "hi"} {
		for _, c := range i64WrapCases {
			cases = append(cases, osdCase{fn: c.name + "_" + half})
		}
	}
	return cases
}

func leb128s64(v int64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		done := v == 0 && b&0x40 == 0 || v == -1 && b&0x40 != 0
		if !done {
			b |= 0x80
		}
		out = append(out, b)
		if done {
			return out
		}
	}
}

func buildI64WrapModule(cases []i64WrapCase) []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Results: []ValType{ValI32}}}))
	var types []int
	var exports []Export
	var bodies [][]byte
	for _, c := range cases {
		for _, half := range []string{"lo", "hi"} {
			code := append([]byte{OpI64Const}, leb128s64(c.a)...)
			code = append(code, OpI64Const)
			code = append(code, leb128s64(c.b)...)
			code = append(code, c.op)
			if half == "hi" {
				code = append(code, OpI64Const, 32, OpI64ShrU)
			}
			code = append(code, OpI32WrapI64)
			exports = append(exports, Export{Name: c.name + "_" + half, Index: len(bodies)})
			types = append(types, 0)
			bodies = append(bodies, buildFuncBody(nil, code))
		}
	}
	w.addSection(3, buildFuncSection(types))
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	return w.bytes()
}

var bareI64Arithmetic = regexp.MustCompile(`(?:g=>)?s\d+_i64 = (?:g=>)?s\d+_i64 [+*-] (?:g=>)?s\d+_i64\.`)
var importingI64Write = regexp.MustCompile(`(?m)^\s*iv_[ab] =`)

func TestI64WrapGeneratedBackends(t *testing.T) {
	for _, c := range i64WrapCases[:3] {
		t.Run(c.name, func(t *testing.T) {
			mod, err := Parse(buildI64WrapModule([]i64WrapCase{c}))
			if err != nil {
				t.Fatal(err)
			}
			name := map[byte]string{OpI64Add: "i64_add", OpI64Sub: "i64_sub", OpI64Mul: "i64_mul"}[c.op]
			for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
				result := mustCompileWith(t, mod, "z_i64wrap", backend, 80)
				all := ""
				for _, src := range result.Files {
					all += src
					checkI64Source(t, src)
				}
				call := name + "( iv_a ="
				header := "METHOD " + name + "."
				if backend != BackendClass {
					call = "PERFORM " + name + " USING"
					header = "FORM " + name + " USING iv_a TYPE int8 iv_b TYPE int8 CHANGING rv TYPE int8."
				}
				if !strings.Contains(all, call) || !strings.Contains(all, header) {
					t.Errorf("%s: missing %s helper/call", backend, name)
				}
				for _, h := range i64HelperOps {
					if h.name != name && strings.Contains(all, h.name) {
						t.Errorf("%s: unused helper %s", backend, h.name)
					}
				}
				if backend == BackendClass && !strings.Contains(all, "METHODS "+name+" IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.") {
					t.Errorf("missing %s signature", name)
				}
			}
			multi := mustCompileMultiClass(t, mod, "z_i64wrap", 80)
			called := false
			for _, src := range multi.ChunkClasses {
				checkI64Source(t, src)
				called = called || strings.Contains(src, "zcl_wasm_rt=>"+name+"( iv_a =")
			}
			if !called {
				t.Errorf("multi: missing %s call", name)
			}
			if !strings.Contains(multi.RuntimeClass, "CLASS-METHODS "+name+" IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.") {
				t.Errorf("multi: missing %s signature", name)
			}
		})
	}
	// All backends must copy the exact shared helper bodies and keep them free
	// of packed arithmetic, parameter writes, integer / and modern syntax.
	for _, h := range i64HelperOps {
		body := i64HelperBody(h.name)
		for _, forbidden := range []string{" TYPE p", " / ", "DATA(", "NEW ", "|", `"`} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q", h.name, forbidden)
			}
		}
		if importingI64Write.MatchString(body) {
			t.Errorf("%s writes IMPORTING parameter", h.name)
		}
		_, templates := runtimeTemplates()
		lines := strings.Split(templates[h.name], "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		if strings.Join(lines, "\n") != body {
			t.Errorf("%s template body differs", h.name)
		}
	}
}

func checkI64Source(t *testing.T, src string) {
	t.Helper()
	if match := bareI64Arithmetic.FindString(src); match != "" {
		t.Errorf("bare i64 arithmetic: %s", match)
	}
	for _, line := range strings.Split(src, "\n") {
		if len(line) > 255 {
			t.Errorf("line exceeds 255 characters: %s", line)
		}
	}
}

func TestI64WrapFixtureResults(t *testing.T) {
	cases := i64WrapOSDCases()
	results := wazeroResults(t, buildI64WrapModule(i64WrapCases), cases)
	for i, c := range i64WrapCases {
		var want int64
		switch c.op {
		case OpI64Add:
			want = c.a + c.b
		case OpI64Sub:
			want = c.a - c.b
		case OpI64Mul:
			want = c.a * c.b
		}
		for half, expected := range []int32{int32(want), int32(want >> 32)} {
			idx := half*len(i64WrapCases) + i
			got := results[idx]
			if got.trap || got.value != expected {
				t.Errorf("%s: got %+v, want %d", cases[idx].fn, got, expected)
			}
		}
	}
}
