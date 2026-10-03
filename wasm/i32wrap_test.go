package wasm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var bareI32Arithmetic = regexp.MustCompile(`(?m)(?:^|\. )\s*(?:g=>)?s\d+ = (?:g=>)?s\d+ [+-/*] (?:g=>)?s\d+\.`)

func TestI32WrapGeneratedBackends(t *testing.T) {
	for _, fixture := range []struct {
		file string
		ops  []string
	}{
		{"add.wasm", []string{"i32_add"}},
		{"factorial.wasm", []string{"i32_sub", "i32_mul"}},
		{"i32wrap", []string{"i32_add", "i32_sub", "i32_mul"}},
	} {
		t.Run(fixture.file, func(t *testing.T) {
			var bin []byte
			var err error
			if fixture.file == "i32wrap" {
				bin = buildI32WrapModule()
			} else {
				bin, err = os.ReadFile(filepath.Join("testdata", fixture.file))
			}
			if err != nil {
				t.Fatal(err)
			}
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
				result := mustCompileWith(t, mod, "z_i32wrap", backend, 80)
				checkI32Sources(t, backend.String(), result.Files, fixture.ops, backend != BackendClass)
				if backend == BackendHybrid && fixture.file == "i32wrap" {
					checkI32Sources(t, "hybrid class", map[string]string{"g": result.Files["LZ_I32WRAP_INTGI.abap"]}, fixture.ops, false)
				}
			}
			multi := mustCompileMultiClass(t, mod, "z_i32wrap", 80)
			checkI32Sources(t, "multi", multi.ChunkClasses, fixture.ops, false)
			for _, src := range []string{multi.RuntimeClass, multi.MainClass} {
				for _, op := range []string{"i32_add", "i32_sub", "i32_mul"} {
					if strings.Contains(src, op) {
						t.Errorf("multi: unused helper %s", op)
					}
				}
			}
		})
	}
}

func checkI32Sources(t *testing.T, backend string, files map[string]string, ops []string, fugr bool) {
	t.Helper()
	all := ""
	for _, src := range files {
		all += src
		if match := bareI32Arithmetic.FindString(src); match != "" {
			t.Errorf("%s: bare i32 arithmetic: %s", backend, match)
		}
	}
	for _, op := range ops {
		if backend == "hybrid" && strings.Contains(all, "lv_w =") {
			checkI32Inline(t, backend, all, op)
			checkI32Helper(t, backend, all, op)
		} else if fugr {
			call := "PERFORM " + op + " USING"
			if !strings.Contains(all, call) {
				t.Errorf("%s: missing call %q", backend, call)
			}
			checkI32Helper(t, backend, all, op)
		} else {
			checkI32Inline(t, backend, all, op)
		}
	}
	for _, src := range files {
		if !strings.Contains(src, "CLASS ") {
			continue
		}
		for _, method := range regexp.MustCompile(`(?s)METHOD \w+\.(.*?)ENDMETHOD\.`).FindAllStringSubmatch(src, -1) {
			count := strings.Count(method[1], "lv_w TYPE int8")
			want := 0
			if strings.Contains(method[1], "lv_w =") {
				want = 1
			}
			if count != want {
				t.Errorf("%s: work variable declared %d times, want %d in %s", backend, count, want, method[0])
			}
		}
		for _, op := range []string{"i32_add", "i32_sub", "i32_mul"} {
			if strings.Contains(src, "METHOD "+op+".") || strings.Contains(src, op+"( iv_a =") {
				t.Errorf("%s: unused class helper %s", backend, op)
			}
			if backend == "hybrid" && strings.Contains(src, "PERFORM "+op+" USING") {
				t.Errorf("hybrid class: helper call for %s", op)
			}
		}
		if backend == "hybrid" && strings.Contains(src, "lv_w =") {
			// The wrapper only delegates; arithmetic lives in extracted block methods.
			for _, op := range ops {
				operator := map[string]string{"i32_add": "+", "i32_sub": "-", "i32_mul": "*"}[op]
				if regexp.MustCompile(`lv_w = (s\d+|lv_w) ` + regexp.QuoteMeta(operator) + ` s\d+\.`).MatchString(src) {
					checkI32Inline(t, "hybrid class", src, op)
				}
			}
		}
	}
}

func checkI32Inline(t *testing.T, backend, src, op string) {
	t.Helper()
	operator := map[string]string{"i32_add": "+", "i32_sub": "-", "i32_mul": "*"}[op]
	wrap := `lv_w = s\d+ ` + regexp.QuoteMeta(operator) + ` s\d+\.\n\s*`
	if op == "i32_mul" {
		wrap = `lv_w = s\d+\.\n\s*lv_w = lv_w \* s\d+\.\n\s*lv_w = lv_w MOD 4294967296\.\n\s*`
	}
	wrap += `IF lv_w > 2147483647\.\n\s*lv_w = lv_w - 4294967296\.\n\s*`
	if op != "i32_mul" {
		wrap += `ELSEIF lv_w < -2147483648\.\n\s*lv_w = lv_w \+ 4294967296\.\n\s*`
	}
	wrap += `ENDIF\.\n\s*s\d+ = lv_w\.`
	if !regexp.MustCompile(wrap).MatchString(src) {
		t.Errorf("%s: missing inline %s wrap", backend, op)
	}
	if !strings.Contains(src, "lv_w TYPE int8") {
		t.Errorf("%s: missing int8 work variable", backend)
	}
	if strings.Contains(src, op+"( iv_a =") {
		t.Errorf("%s: helper call for %s", backend, op)
	}
}

func checkI32Helper(t *testing.T, backend, src, op string) {
	t.Helper()
	operator := map[string]string{"i32_add": "+", "i32_sub": "-", "i32_mul": "*"}[op]
	start := "METHOD " + op + "."
	end := "ENDMETHOD."
	if backend == "fugr" || backend == "hybrid" {
		start = "\nFORM " + op + " USING"
		end = "ENDFORM."
	}
	idx := strings.Index(src, start)
	if idx < 0 {
		t.Errorf("%s: missing %s helper", backend, op)
		return
	}
	body := src[idx:]
	if n := strings.Index(body, end); n >= 0 {
		body = body[:n]
	}
	for _, line := range []string{
		"DATA lv_p TYPE int8.",
		"lv_p = iv_a.",
		"lv_p = lv_p " + operator + " iv_b.",
		"lv_p = lv_p MOD 4294967296.",
		"IF lv_p >= 2147483648.",
		"lv_p = lv_p - 4294967296.",
		"rv = lv_p.",
	} {
		if !strings.Contains(body, line) {
			t.Errorf("%s %s: missing %q", backend, op, line)
		}
	}
}

// buildI32WrapModule supplies direct arithmetic exports for OSD boundary cases.
// Blocks also exercise Hybrid's class methods while FUGR keeps PERFORM.
func buildI32WrapModule() []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Params: []ValType{ValI32, ValI32}, Results: []ValType{ValI32}}}))
	w.addSection(3, buildFuncSection([]int{0, 0, 0, 0}))
	w.addSection(7, buildExportSection([]Export{
		{Name: "add", Kind: 0, Index: 0},
		{Name: "sub", Kind: 0, Index: 1},
		{Name: "mul", Kind: 0, Index: 2},
		{Name: "identity", Kind: 0, Index: 3},
	}))
	var bodies [][]byte
	for _, op := range []byte{OpI32Add, OpI32Sub, OpI32Mul} {
		bodies = append(bodies, buildFuncBody(nil, []byte{OpBlock, 0x7f, OpLocalGet, 0, OpLocalGet, 1, op, OpEnd}))
	}
	bodies = append(bodies, buildFuncBody(nil, []byte{OpLocalGet, 0}))
	w.addSection(10, buildCodeSection(bodies))
	return w.bytes()
}
