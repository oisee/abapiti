package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/abapsize"
)

// longExportName is longer than an ABAP identifier, so it reaches the 30-character cap.
const longExportName = "a_very_long_exported_function_name_that_exceeds_identifiers"

// buildManyParamsWasm builds one exported (i32 × n) -> i32 function that calls
// itself with all of its parameters: its signature and its call both list n
// parameters, which no single 255-character line can hold for large n.
func buildManyParamsWasm(n int) []byte {
	params := make([]ValType, n)
	for i := range params {
		params[i] = ValI32
	}
	var code []byte
	for i := 0; i < n; i++ {
		code = append(code, OpLocalGet)
		code = append(code, leb128u(uint32(i))...)
	}
	code = append(code, OpCall, 0x00)
	return buildSingleFuncWasm(longExportName, FuncType{Params: params, Results: []ValType{ValI32}}, nil, code)
}

type lineLimitModule struct {
	name string
	mod  *Module
}

// lineLimitModules returns every testdata fixture (QuickJS included) plus the
// synthetic many-parameter modules.
func lineLimitModules(t *testing.T) []lineLimitModule {
	t.Helper()
	fixtures, err := filepath.Glob("testdata/*.wasm")
	if err != nil {
		t.Fatal(err)
	}
	var mods []lineLimitModule
	for _, path := range fixtures {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		mod, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		mods = append(mods, lineLimitModule{filepath.Base(path), mod})
	}
	for _, n := range []int{24, 40} {
		mod, err := Parse(buildManyParamsWasm(n))
		if err != nil {
			t.Fatal(err)
		}
		mods = append(mods, lineLimitModule{fmt.Sprintf("params%d", n), mod})
	}
	return mods
}

// forEachBackend compiles every module with all four backends and hands the
// generated files to check.
func forEachBackend(t *testing.T, check func(t *testing.T, files map[string]string)) {
	for _, m := range lineLimitModules(t) {
		t.Run(m.name, func(t *testing.T) {
			for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
				t.Run(backend.String(), func(t *testing.T) {
					check(t, CompileWith(m.mod, "zcl_line_limit", backend, 80).Files)
				})
			}
			t.Run("multi-class", func(t *testing.T) {
				result := mustCompileMultiClass(t, m.mod, "zcl_line_limit", 80)
				files := result.Files("zcl_line_limit")
				check(t, files)
			})
		})
	}
}

func TestGeneratedLineLimit(t *testing.T) {
	forEachBackend(t, checkLineLimit)
}

func checkLineLimit(t *testing.T, files map[string]string) {
	t.Helper()
	r := abapsize.Report(files)
	if len(r.LongLines) > 0 {
		errors := r.Errors()
		if len(errors) > 10 {
			errors = errors[:10]
		}
		t.Fatalf("%d lines exceed 255 characters: %v", len(r.LongLines), errors)
	}
}

// TestManyParamsWrapped checks that the many-parameter module really exercises
// wrapping: its signature and self-call must survive, split across lines.
func TestManyParamsWrapped(t *testing.T) {
	mod, err := Parse(buildManyParamsWasm(40))
	if err != nil {
		t.Fatal(err)
	}
	src := Compile(mod, "zcl_line_limit")
	flat := strings.Join(strings.Fields(src), " ")
	name := sanitizeABAP(longExportName)
	for _, want := range []string{
		"METHODS " + name + " IMPORTING p0 TYPE i",
		"p39 TYPE i RETURNING",
		name + "( p0 = ",
		"p39 = ",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("missing %q in generated class", want)
		}
	}
}

// commentWithCode matches a `"` comment whose text contains a statement end,
// i.e. code that the comment swallowed.
var commentWithCode = regexp.MustCompile(`\.(\s|$)`)

func TestNoCodeAfterComment(t *testing.T) {
	forEachBackend(t, func(t *testing.T, files map[string]string) {
		t.Helper()
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		var bad []string
		for _, name := range names {
			for i, line := range strings.Split(files[name], "\n") {
				at := commentStart(line)
				if at >= 0 && commentWithCode.MatchString(line[at+1:]) {
					bad = append(bad, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
				}
			}
		}
		if len(bad) > 0 {
			shown := bad
			if len(shown) > 5 {
				shown = shown[:5]
			}
			t.Fatalf("%d lines have code after a comment: %v", len(bad), shown)
		}
	})
}

func TestCommentStart(t *testing.T) {
	for _, tc := range []struct {
		line string
		want int
	}{
		{`x = 1.`, -1},
		{`x = 1. " c`, 7},
		{`x = '"'. " c`, 9},
		{`x = 'it''s"'.`, -1},
		{"x = `\"`. \" c", 9},
		{`* full "line" comment`, 0},
	} {
		if got := commentStart(tc.line); got != tc.want {
			t.Errorf("commentStart(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

func TestWrapLongLines(t *testing.T) {
	long := "  CALL METHOD x EXPORTING " + strings.Repeat("parameter = 'a b c' ", 20) + ". \" a comment"
	got := wrapLongLines(long + "\nshort.\n")
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if lines[len(lines)-1] != "short." {
		t.Fatalf("short line changed: %q", got)
	}
	for _, line := range lines {
		if len(line) > abapLineLimit {
			t.Errorf("line still %d long: %q", len(line), line)
		}
		if strings.Count(line, "'")%2 != 0 {
			t.Errorf("literal split across lines: %q", line)
		}
	}
	if strings.Join(strings.Fields(got), " ") != strings.Join(strings.Fields(long+" short."), " ") {
		t.Fatalf("tokens changed:\n%s", got)
	}
	if !strings.HasSuffix(lines[len(lines)-2], `" a comment`) {
		t.Fatalf("comment not kept at the end of the statement: %q", lines[len(lines)-2])
	}
}

func buildFloatConstWasm(f64 float64, f32 float32) []byte {
	code := []byte{OpF64Const}
	code = binary.LittleEndian.AppendUint64(code, math.Float64bits(f64))
	code = append(code, OpDrop, OpF32Const)
	code = binary.LittleEndian.AppendUint32(code, math.Float32bits(f32))
	code = append(code, OpDrop)
	return buildSingleFuncWasm("floats", FuncType{}, nil, code)
}

// TestFloatConstLiterals pins float constants to the generator's fixed-point
// "%f" form; only magnitudes too large for one line use exponent notation.
func TestFloatConstLiterals(t *testing.T) {
	for _, tc := range []struct {
		f64    float64
		f32    float32
		want64 string
		want32 string
	}{
		{1.5, 0.25, "'1.500000'", "'0.250000'"},
		{-2, 3, "'-2.000000'", "'3.000000'"},
		{1e300, math.MaxFloat32, "'1e+300'", "'340282346638528859811704183484516925440.000000'"},
	} {
		mod, err := Parse(buildFloatConstWasm(tc.f64, tc.f32))
		if err != nil {
			t.Fatal(err)
		}
		src := Compile(mod, "zcl_floats")
		for _, want := range []string{tc.want64, tc.want32} {
			if !strings.Contains(src, " = "+want+".") {
				t.Errorf("missing literal %s", want)
			}
		}
		checkLineLimit(t, map[string]string{"zcl_floats.clas.abap": src})
	}
}
