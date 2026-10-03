package wasm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var kernelScopeRE = regexp.MustCompile(`(?is)\b(?:METHOD|FORM|FUNCTION)\s+(\w+(?:~\w+)?)([^.]*)\.(.*?)\b(?:ENDMETHOD|ENDFORM|ENDFUNCTION)\.`)
var kernelSignatureRE = regexp.MustCompile(`(?is)\b(?:CLASS-)?METHODS\s+(\w+)\s+([^.]*)\.`)
var kernelTypeRE = regexp.MustCompile(`(?i)\b(\w+)\s+TYPE\s+(i|int8|p|xstring|string|x|f)\b`)
var kernelValueRE = regexp.MustCompile(`(?i)VALUE\((\w+)\)`)
var kernelInlineRE = regexp.MustCompile(`(?i)DATA\((\w+)\)\s*=\s*(?:CONV\s+(i|int8|p|xstring|string|x|f)\s*\(|(\w+))`)
var kernelBitRE = regexp.MustCompile(`(?i)(\w+)\s+BIT-(?:AND|OR|XOR)\s+(\w+)|BIT-NOT\s+(\w+)`)
var kernelWriterRE = regexp.MustCompile(`(?i)\b(\w+)\s*\+\s*[^.\n=]+\([^).\n]+\)\s*=`)

func kernelInvalidPatterns(src string) []string {
	src = strings.ToLower(stripABAPComments(src))
	typed := func(src string, types map[string]string) {
		src = kernelValueRE.ReplaceAllString(src, "$1")
		for _, m := range kernelTypeRE.FindAllStringSubmatch(src, -1) {
			types[m[1]] = m[2]
		}
		for _, m := range kernelInlineRE.FindAllStringSubmatch(src, -1) {
			if m[2] != "" {
				types[m[1]] = m[2]
			} else if typ := types[m[3]]; typ != "" {
				types[m[1]] = typ
			}
		}
	}
	globals := map[string]string{}
	typed(kernelSignatureRE.ReplaceAllString(kernelScopeRE.ReplaceAllString(src, ""), ""), globals)
	signatures := map[string]string{}
	for _, m := range kernelSignatureRE.FindAllStringSubmatch(src, -1) {
		signatures[m[1]] = m[2]
	}
	var bad []string
	for _, scope := range kernelScopeRE.FindAllStringSubmatch(src, -1) {
		types := map[string]string{}
		for name, typ := range globals {
			types[name] = typ
		}
		method := scope[1]
		if _, suffix, ok := strings.Cut(method, "~"); ok {
			method = suffix
		}
		typed(signatures[method], types)
		typed(scope[2], types)
		typed(scope[3], types)
		for _, m := range kernelBitRE.FindAllStringSubmatch(scope[3], -1) {
			for _, operand := range m[1:] {
				if typ := types[operand]; typ == "i" || typ == "int8" || typ == "p" {
					bad = append(bad, fmt.Sprintf("%s: numeric BIT operand %s (%s): %s", scope[1], operand, typ, m[0]))
				}
			}
		}
		for _, m := range kernelWriterRE.FindAllStringSubmatch(scope[3], -1) {
			if typ := types[m[1]]; typ == "xstring" || typ == "string" {
				bad = append(bad, fmt.Sprintf("%s: %s writer: %s", scope[1], typ, m[0]))
			}
		}
	}
	return bad
}

func TestKernelInvalidPatternScanner(t *testing.T) {
	invalid := `METHODS test IMPORTING iv TYPE int8 CHANGING cv TYPE xstring.
METHOD test.
DATA lv TYPE p LENGTH 16 DECIMALS 0.
DATA text TYPE string.
DATA(bits) = CONV i( 1 ).
lv = iv BIT-AND lv.
bits = BIT-NOT bits.
cv+off(n) = '00'.
text+0(1) = 'x'.
ENDMETHOD.`
	if bad := kernelInvalidPatterns(invalid); len(bad) != 5 {
		t.Fatalf("scanner: %v", bad)
	}
	valid := `METHOD test. DATA bits TYPE x LENGTH 8. DATA mem TYPE xstring.
bits = bits BIT-XOR bits. bits+0(1) = '00'.
bits = mem+off(n). REPLACE SECTION OFFSET off LENGTH n OF mem WITH bits IN BYTE MODE. ENDMETHOD.`
	if bad := kernelInvalidPatterns(valid); len(bad) != 0 {
		t.Fatal(bad)
	}
}

func TestRuntimeHelpersKernelValid(t *testing.T) {
	declarations, bodies := runtimeTemplates()
	for name, body := range bodies {
		src := declarations[name] + "\nMETHOD " + name + ".\n" + body + "\nENDMETHOD."
		if bad := kernelInvalidPatterns(src); len(bad) > 0 {
			t.Errorf("%s: %v", name, bad)
		}
		if strings.Contains(body, "TYPE p ") {
			t.Errorf("%s uses packed arithmetic", name)
		}
	}
	if bad := kernelInvalidPatterns(emitRuntimeClass()); len(bad) > 0 {
		t.Fatal(bad)
	}
}

func TestGeneratedCodeKernelValid(t *testing.T) {
	forEachBackend(t, func(t *testing.T, files map[string]string) {
		var names []string
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		var src strings.Builder
		for _, name := range names {
			src.WriteString(files[name])
			src.WriteByte('\n')
		}
		if bad := kernelInvalidPatterns(src.String()); len(bad) > 0 {
			if len(bad) > 10 {
				bad = bad[:10]
			}
			t.Fatal(bad)
		}
	})
}

func TestFloatRuntimeZeroStoresAndNonfiniteTraps(t *testing.T) {
	_, bodies := runtimeTemplates()
	for _, width := range []string{"32", "64"} {
		store := bodies["mem_st_f"+width]
		if strings.Contains(store, "RETURN.") || !strings.Contains(store, "IF lv_mag = 0.\nlv_value = 0.\nELSE.") {
			t.Errorf("f%s store must write zero without returning: %s", width, store)
		}
		for _, name := range []string{"mem_ld_f" + width, "reinterpret_i" + width + "_f" + width} {
			body := bodies[name]
			exponent := "255"
			if width == "64" {
				exponent = "2047"
			}
			guard := "IF lv_exp = " + exponent + ". " + wasmTrap + " ENDIF."
			guardAt := strings.Index(body, guard)
			fractionAt := strings.Index(body, "lv_frac = lv_bits MOD")
			if guardAt < 0 || fractionAt < guardAt {
				t.Errorf("%s must trap before decoding nonfinite fields: %s", name, body)
			}
		}
	}
}

func TestMemoryFillBuildsRangeBeforeReplacing(t *testing.T) {
	_, bodies := runtimeTemplates()
	body := bodies["mem_fill"]
	if strings.Count(body, "REPLACE SECTION") != 1 || strings.Contains(body, "DO iv_n TIMES") ||
		!strings.Contains(body, "CONCATENATE lv_fill lv_fill INTO lv_fill IN BYTE MODE.") ||
		!strings.Contains(body, "REPLACE SECTION OFFSET iv_dst LENGTH iv_n OF cv_mem WITH lv_fill IN BYTE MODE.") {
		t.Fatalf("memory.fill must build the range by doubling and replace once: %s", body)
	}
}

func TestSingleClassMemoryHelpersOwnBuffer(t *testing.T) {
	modules := lineLimitModules(t)
	bin, _ := runtimeFixture()
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	modules = append(modules, lineLimitModule{"runtime_helpers", mod})
	actualMemory := regexp.MustCompile(`(?i)=\s*mv_mem\b`)
	parameterLists := regexp.MustCompile(`(?is)\b[a-z][a-z0-9_]*\s*\(([^.]*)\)`)
	xstringParam := regexp.MustCompile(`(?i)\bTYPE\s+xstring\b`)
	sharedDeclarations, _ := runtimeTemplates()
	for _, m := range modules {
		t.Run(m.name, func(t *testing.T) {
			src := mustCompileWith(t, m.mod, "zcl_owned_memory", BackendClass, 80).Files["zcl_owned_memory.clas.abap"]
			if m.name == "runtime_helpers" {
				src = runtimeFixtureClass(m.mod, "zcl_owned_memory")
			}
			// Named actual parameters use '='; ordinary reads and assignments do not.
			for _, call := range parameterLists.FindAllStringSubmatch(src, -1) {
				if actualMemory.MatchString(call[1]) {
					t.Errorf("mv_mem passed as an actual parameter: %s", call[0])
				}
			}
			for _, signature := range kernelSignatureRE.FindAllStringSubmatch(src, -1) {
				params := strings.Split(signature[2], "RETURNING")[0]
				if xstringParam.MatchString(params) {
					t.Errorf("%s declares an xstring memory parameter: %s", signature[1], params)
				}
				if strings.HasPrefix(signature[1], "mem_") && strings.HasPrefix(strings.TrimSpace(signature[0]), "CLASS-METHODS") {
					t.Errorf("%s must be an instance method", signature[1])
				}
			}
			for _, method := range runtimeMethodRE.FindAllStringSubmatch(src, -1) {
				if strings.Contains(sharedDeclarations[method[1]], "TYPE xstring") && method[1] != "alloc_mem" && method[1] != "mem_zero_pages" {
					if !strings.Contains(method[2], "mv_mem") || runtimeMemoryRE.MatchString(method[2]) {
						t.Errorf("%s must access mv_mem directly", method[1])
					}
				}
			}
			checkLineLimit(t, map[string]string{"class": src})
			if bad := kernelInvalidPatterns(src); len(bad) > 0 {
				t.Fatal(bad)
			}
		})
	}
}

func TestSplitRuntimeHelpersKernelValid(t *testing.T) {
	bin, _ := runtimeFixture()
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCompileMultiClass(t, mod, "zcl_split_kernel", 200)
	assertSplitMemoryPrivate(t, r)
	files := mustSplitFiles(t, r, "zcl_split_kernel")
	checkLineLimit(t, files)
	var all strings.Builder
	for name, src := range files {
		assertNoABAPComments(t, name, src)
		if parameterWrite.MatchString(src) {
			t.Errorf("%s writes to IMPORTING parameters", name)
		}
		all.WriteString(src)
		all.WriteByte('\n')
	}
	if bad := kernelInvalidPatterns(all.String()); len(bad) > 0 {
		t.Fatal(bad)
	}
	_, bodies := singleClassRuntimeTemplates()
	found := 0
	for _, method := range runtimeMethodRE.FindAllStringSubmatch(r.StateClass, -1) {
		if body, ok := bodies[method[1]]; ok {
			found++
			if strings.Join(strings.Fields(method[2]), " ") != strings.Join(strings.Fields(body), " ") {
				t.Errorf("split helper %s differs from the shared kernel-valid body", method[1])
			}
		}
	}
	if found < 20 {
		t.Fatalf("fixture only exercised %d runtime helpers", found)
	}
}
