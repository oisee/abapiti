package wasm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var bareI32Arithmetic = regexp.MustCompile(`(?m)^\s*(?:g=>)?s\d+ = (?:g=>)?s\d+ [+-/*] (?:g=>)?s\d+\.$`)

func TestI32WrapGeneratedBackends(t *testing.T) {
	for _, fixture := range []struct {
		file string
		ops  []string
	}{
		{"add.wasm", []string{"i32_add"}},
		{"factorial.wasm", []string{"i32_sub", "i32_mul"}},
	} {
		t.Run(fixture.file, func(t *testing.T) {
			bin, err := os.ReadFile(filepath.Join("testdata", fixture.file))
			if err != nil {
				t.Fatal(err)
			}
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
				result := CompileWith(mod, "z_i32wrap", backend, 80)
				checkI32Sources(t, backend.String(), result.Files, fixture.ops, backend != BackendClass)
			}
			multi := CompileMultiClass(mod, "z_i32wrap", 80)
			checkI32Sources(t, "multi", multi.ChunkClasses, fixture.ops, false)
			for _, op := range fixture.ops {
				if !strings.Contains(multi.RuntimeClass, "CLASS-METHODS "+op+" IMPORTING") {
					t.Errorf("multi: missing runtime declaration for %s", op)
				}
				checkI32Helper(t, "multi runtime", multi.RuntimeClass, op)
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
		var call string
		switch backend {
		case "multi":
			call = "zcl_wasm_rt=>" + op + "( iv_a ="
		case "fugr", "hybrid":
			call = "PERFORM " + op + " USING"
		default:
			call = op + "( iv_a ="
		}
		if !strings.Contains(all, call) {
			t.Errorf("%s: missing call %q", backend, call)
		}
		if backend != "multi" {
			found := false
			for _, src := range files {
				if strings.Contains(src, "METHOD "+op+".") || strings.Contains(src, "\nFORM "+op+" USING") {
					found = true
					checkI32Helper(t, backend, src, op)
				}
			}
			if !found {
				t.Errorf("%s: missing %s helper", backend, op)
			}
		}
		if fugr && !strings.Contains(all, "FORM "+op+" USING iv_a TYPE i iv_b TYPE i CHANGING rv TYPE i.") {
			t.Errorf("%s: missing FORM declaration for %s", backend, op)
		}
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
