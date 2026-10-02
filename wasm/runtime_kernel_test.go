package wasm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var kernelScopeRE = regexp.MustCompile(`(?is)\b(?:METHOD|FORM|FUNCTION)\s+(\w+)([^.]*)\.(.*?)\b(?:ENDMETHOD|ENDFORM|ENDFUNCTION)\.`)
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
		typed(signatures[scope[1]], types)
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
