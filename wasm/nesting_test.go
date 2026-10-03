package wasm

import (
	"regexp"
	"strings"
	"testing"
)

const deepSwitchBlocks = 300

func buildDeepSwitchWasm() []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Params: []ValType{ValI32}, Results: []ValType{ValI32}}}))
	names := deepSwitchNames
	var exports []Export
	var types []int
	var bodies [][]byte
	for n, name := range names {
		exports = append(exports, Export{Name: name, Kind: 0, Index: n})
		types = append(types, 0)
		var code []byte
		constant := func(v int) { code = append(code, OpI32Const); code = append(code, leb128s(int32(v))...) }
		branch := func(op byte, depth int) { code = append(code, op); code = append(code, leb128u(uint32(depth))...) }
		table := func(labels []int, def int) {
			code = append(code, OpBrTable)
			code = append(code, leb128u(uint32(len(labels)))...)
			for _, d := range append(labels, def) {
				code = append(code, leb128u(uint32(d))...)
			}
		}
		result := name == "results300" || name == "i64results"
		wide := name == "i64results"
		restart := name == "restart300"
		constant(-1)
		code = append(code, OpLocalSet, 1, OpBlock, 0x40, OpNop)
		if restart {
			code = append(code, OpLoop, 0x40)
		}
		if result {
			constant(99)
		}
		for j := 0; j < deepSwitchBlocks; j++ {
			typ := byte(0x40)
			if result {
				typ = 0x7f
				if wide {
					typ = 0x7e
				}
			}
			code = append(code, OpBlock, typ)
		}
		if result {
			if wide {
				code = append(code, OpI64Const)
				code = append(code, leb128s(700)...)
				code = append(code, OpI64Const)
				code = append(code, leb128s(77)...)
			} else {
				constant(700)
				constant(77)
			}
		}
		code = append(code, OpLocalGet, 0)
		labels := make([]int, deepSwitchBlocks)
		for j := range labels {
			labels[j] = j
		}
		def := deepSwitchBlocks
		if name == "flatdefault" || result {
			def--
		}
		if restart {
			def++
		}
		table(labels, def)
		for j := 0; j < deepSwitchBlocks; j++ {
			code = append(code, OpEnd)
			if result {
				if wide {
					code = append(code, OpI64Const, 1, OpI64Add)
				} else {
					constant(1)
					code = append(code, OpI32Add)
				}
				continue
			}
			remaining := deepSwitchBlocks - 1 - j
			if name == "nested300" && j == 129 {
				for k := 0; k < 65; k++ {
					code = append(code, OpBlock, 0x7f)
				}
				constant(88)
				branch(OpBr, 64)
				for k := 0; k < 65; k++ {
					code = append(code, OpEnd)
				}
				code = append(code, OpDrop)
			}
			if name == "loopcase" {
				constant(0)
				code = append(code, OpLocalSet, 2, OpLoop, 0x40, OpLocalGet, 2)
				constant(1)
				code = append(code, OpI32Add, OpLocalTee, 2)
				constant(3)
				code = append(code, OpI32LtS)
				branch(OpBrIf, 0)
				code = append(code, OpEnd)
			}
			constant(j)
			code = append(code, OpLocalSet, 1)
			if name == "escape300" {
				constant(1)
				code = append(code, OpIf, 0x40, OpLoop, 0x40)
				constant(1)
				branch(OpBrIf, remaining+2)
				code = append(code, OpEnd, OpEnd)
			}
			if restart {
				code = append(code, OpLocalGet, 2)
				constant(1)
				code = append(code, OpI32Add, OpLocalTee, 2)
				constant(3)
				code = append(code, OpI32LtS)
				branch(OpBrIf, remaining)
				branch(OpBr, remaining+1)
			} else if name == "mixed300" && remaining > 0 {
				constant(0)
				code = append(code, OpLocalSet, 2, OpLoop, 0x40, OpLocalGet, 2)
				constant(1)
				code = append(code, OpI32Add, OpLocalTee, 2)
				constant(1)
				code = append(code, OpI32Sub)
				table([]int{0, 1}, remaining+1)
				code = append(code, OpEnd)
			} else {
				branch(OpBr, remaining)
			}
		}
		if result {
			if wide {
				code = append(code, OpI32WrapI64)
			}
			code = append(code, OpLocalSet, 1, OpDrop)
		}
		if restart {
			code = append(code, OpEnd)
		}
		code = append(code, OpEnd, OpLocalGet, 1)
		bodies = append(bodies, buildFuncBody([]ValType{ValI32, ValI32}, code))
	}
	w.addSection(3, buildFuncSection(types))
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	return w.bytes()
}

var deepSwitchNames = []string{"switch300", "flatdefault", "results300", "i64results", "loopcase", "escape300", "restart300", "mixed300", "nested300"}

func deepSwitchCases() []osdCase {
	var cases []osdCase
	for _, name := range deepSwitchNames {
		for i := int32(-1); i <= deepSwitchBlocks+1; i++ {
			cases = append(cases, osdCase{name, []int32{i}})
		}
	}
	return cases
}

var nestingToken = regexp.MustCompile(`(?i)\b(IF|DO|WHILE|CASE|TRY|LOOP|ENDIF|ENDDO|ENDWHILE|ENDCASE|ENDTRY|ENDLOOP)\b`)

func assertNestingBound(t *testing.T, name, src string) {
	t.Helper()
	for _, scope := range kernelScopeRE.FindAllStringSubmatch(src, -1) {
		depth, peak := 0, 0
		for _, token := range nestingToken.FindAllString(scope[3], -1) {
			if strings.HasPrefix(strings.ToUpper(token), "END") {
				depth--
			} else {
				depth++
				peak = max(peak, depth)
			}
			if depth < 0 {
				t.Fatalf("%s/%s: unbalanced structures", name, scope[1])
			}
		}
		if depth != 0 || peak > 100 {
			t.Fatalf("%s/%s: depth=%d maximum=%d", name, scope[1], depth, peak)
		}
	}
	if hasLongLine(src) {
		t.Errorf("%s: long line", name)
	}
	if bad := kernelInvalidPatterns(src); len(bad) > 0 {
		t.Errorf("%s: %v", name, bad)
	}
	if parameterWrite.MatchString(src) {
		t.Errorf("%s: IMPORTING parameter write", name)
	}
	if stripABAPComments(src) != src {
		t.Errorf("%s: comments", name)
	}
}

func TestDeepSwitchNesting(t *testing.T) {
	bin := buildDeepSwitchWasm()
	cases := deepSwitchCases()
	values := wazeroResults(t, bin, cases)
	for i, tc := range cases {
		input := tc.args[0]
		want := input
		if input < 0 || input >= deepSwitchBlocks {
			want = -1
		}
		switch tc.fn {
		case "flatdefault":
			if want == -1 {
				want = deepSwitchBlocks - 1
			}
		case "results300", "i64results":
			if want == -1 {
				want = deepSwitchBlocks - 1
			}
			want = 77 + deepSwitchBlocks - want
		case "mixed300":
			if want >= 0 {
				want = deepSwitchBlocks - 1
			}
		}
		if values[i].trap || values[i].value != want {
			t.Fatalf("%s(%d): %v, want %d", tc.fn, input, values[i], want)
		}
	}
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
		t.Run(backend.String(), func(t *testing.T) {
			output := mustCompileWith(t, mod, "zcl_deep", backend, 2)
			for name, src := range output.Files {
				assertNestingBound(t, name, src)
			}
		})
	}
	output := mustCompileMultiClass(t, mod, "zcl_deep", 2)
	assertNestingBound(t, "main", output.MainClass)
	assertNestingBound(t, "state", output.StateClass)
	for name, src := range output.Interfaces {
		assertNestingBound(t, name, src)
	}
	for name, src := range output.ChunkClasses {
		assertNestingBound(t, name, src)
	}
}

func TestNestingGuard(t *testing.T) {
	for _, opcode := range []byte{OpIf, OpLoop, OpBlock} {
		w := newWasmBuilder()
		w.addSection(1, buildTypeSection([]FuncType{{}}))
		w.addSection(3, buildFuncSection([]int{0}))
		var code []byte
		for i := 0; i < 101; i++ {
			if opcode == OpIf {
				code = append(code, OpI32Const, 1)
			}
			code = append(code, opcode, 0x40, OpNop)
		}
		for i := 0; i < 101; i++ {
			code = append(code, OpEnd)
		}
		w.addSection(10, buildCodeSection([][]byte{buildFuncBody(nil, code)}))
		mod, err := Parse(w.bytes())
		if err != nil {
			t.Fatal(err)
		}
		source, err := Compile(mod, "zcl_guard")
		if err == nil || !strings.Contains(err.Error(), "nesting depth 101 exceeds limit 100 in f0") {
			t.Fatalf("opcode %x: %v", opcode, err)
		}
		if source != "" {
			t.Fatal("rejected source returned to caller")
		}
		_, err = CompileMultiClass(mod, "zcl_guard", 1)
		if err == nil {
			t.Fatalf("opcode %x: missing multi-class guard", opcode)
		}
		for _, backend := range []BackendKind{BackendFUGR, BackendHybrid} {
			_, err = CompileWith(mod, "zcl_guard", backend, 1)
			if opcode == OpIf && err == nil {
				t.Fatalf("%s: missing guard", backend)
			}
			if opcode != OpIf && err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, depth := range []int{100, 101} {
		src := "METHOD test. text = 'IF. DO. ''TRY.'''. text = |CASE. LOOP.|.\n\" IF.\n* DO.\n" + strings.Repeat("DO 1 TIMES. IF 1 = 1. ENDIF. ENDDO. ", 10) + strings.Repeat("DO. ", depth) + strings.Repeat("ENDDO. ", depth) + "ENDMETHOD."
		err := checkABAPNesting(src)
		if (err != nil) != (depth > 100) {
			t.Fatalf("depth %d: %v", depth, err)
		}
	}
}
