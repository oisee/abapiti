package wasm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// osdCase is one call of an exported WASM function. The expected value is not
// written here: it is computed by running the same .wasm in wazero, so the
// generated ABAP Unit test checks the ABAP against an independent engine.
// When wazero traps, the ABAP call must raise an exception. Non-finite float
// constants have explicit ABAP trap expectations because TYPE f cannot hold them.
type osdCase struct {
	fn   string
	args []int32
}

// osdResult is wazero's outcome of one case: a value, or a trap.
type osdResult struct {
	value int32
	trap  bool
}

// osdModules are the M1 modules deployed to open-steamgate. Wrapping inputs
// exercise the generated int8 i32 arithmetic on the ABAP runtime.
var osdModules = []struct {
	file  string
	class string
	cases []osdCase
}{
	{"add.wasm", "zcl_abapiti_add", []osdCase{
		{"add", []int32{2, 3}},
		{"add", []int32{0, 0}},
		{"add", []int32{-1, 1}},
		{"add", []int32{-5, -7}},
		{"add", []int32{2147483647, 0}},
		{"add", []int32{2147483647, 1}},
		{"add", []int32{-2147483648, -1}},
	}},
	{"factorial.wasm", "zcl_abapiti_factorial", []osdCase{
		{"factorial", []int32{0}},
		{"factorial", []int32{1}},
		{"factorial", []int32{5}},
		{"factorial", []int32{10}},
		{"factorial", []int32{12}},
		{"factorial", []int32{13}},
		{"factorial", []int32{20}},
	}},
	{"memgrow", "zcl_abapiti_memgrow", []osdCase{
		{"grow", []int32{1}},
		{"grow", []int32{1}},
		{"grow", []int32{-1}},
		{"size", nil},
		{"storeload", nil},
	}},
	{"memzero", "zcl_abapiti_memzero", []osdCase{
		{"size", nil},
	}},
	// call_indirect through a table whose segment starts at 1 (as clang
	// emits): slot 0 is null, slot 4 has another type, slot 5 an identical
	// type under another index, 6 is out of range.
	{"callind", "zcl_abapiti_callind", []osdCase{
		{"call", []int32{1, 10}},
		{"call", []int32{2, 10}},
		{"call", []int32{3, 10}},
		{"call", []int32{5, 10}},
		{"call", []int32{0, 10}},
		{"call", []int32{4, 10}},
		{"call", []int32{6, 10}},
		{"call", []int32{-1, 10}},
		{"call", []int32{2, 7}},
	}},
	// Sign-extending i64 loads and arithmetic right shifts.
	{"helpers", "zcl_abapiti_helpers", []osdCase{
		{"l8s", []int32{0}},
		{"l8s", []int32{1}},
		{"l8u", []int32{0}},
		{"l16s", []int32{2}},
		{"l16s", []int32{4}},
		{"l16u", []int32{2}},
		{"l32s", []int32{6}},
		{"l32s_hi", []int32{6}},
		{"l32s_hi", []int32{10}},
		{"l32u_hi", []int32{6}},
		{"sar32", []int32{-5, 1}},
		{"sar32", []int32{-5, 31}},
		{"sar32", []int32{5, 31}},
		{"sar32", []int32{-2147483648, 31}},
		{"sar32", []int32{-8, 33}},
		{"sar32", []int32{100, 2}},
		{"sar64", []int32{-5, 63}},
		{"sar64", []int32{-5, 1}},
		{"sar64", []int32{7, 63}},
		{"sar64_hi", []int32{-5, 63}},
		{"sar64_hi", []int32{-5, 33}},
		{"sar64_hi", []int32{1, 32}},
	}},
	{"truncsat", "zcl_abapiti_truncsat", truncSatOSDCases()},
	{"floattrap", "zcl_abapiti_floattrap", specialFloatOSDCases()},
	{"i64wrap", "zcl_abapiti_i64wrap", i64WrapOSDCases()},
	{"i32wrap", "zcl_abapiti_i32wrap", []osdCase{
		{"add", []int32{2147483647, 1}},
		{"add", []int32{-2147483648, -1}},
		{"add", []int32{2147483647, 2147483647}},
		{"add", []int32{-2147483648, -2147483648}},
		{"sub", []int32{-2147483648, 1}},
		{"sub", []int32{2147483647, -1}},
		{"sub", []int32{2147483647, -2147483648}},
		{"sub", []int32{-2147483648, 2147483647}},
		{"mul", []int32{2147483647, 2147483647}},
		{"mul", []int32{-2147483648, -2147483648}},
		{"mul", []int32{-2147483648, -1}},
		{"mul", []int32{-2147483648, 2147483647}},
		{"mul", []int32{-7, 3}},
		{"mul", []int32{-7, 0}},
	}},
	// Last: compileCFixture skips the whole test when clang is missing (as on
	// the OSD runner), so no module after it would be generated there.
	{"corpus", "zcl_abapiti_corpus", []osdCase{
		{"factorial", []int32{5}},
		{"fibonacci", []int32{10}},
		{"and_", []int32{13, 11}},
		{"shl", []int32{3, 4}},
		{"shr", []int32{-16, 2}},
		{"shr_u", []int32{-1, 1}},
		{"unsigned_lt", []int32{-1, 1}},
		{"add64", []int32{100, 23}},
	}},
	{"branches", "zcl_abapiti_branches", branchLoopCases},
	{"suite", "zcl_abapiti_suite", func() []osdCase {
		var cases []osdCase
		for _, c := range suiteTestCases {
			cases = append(cases, osdCase{c.FuncName, c.Args})
		}
		return cases
	}()},
}

// wazeroResults runs each case through wazero and returns the i32 results
// or traps. The instance is kept across a trap, as the ABAP object is.
func wazeroResults(t *testing.T, bin []byte, cases []osdCase) []osdResult {
	t.Helper()
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	mod, err := rt.Instantiate(ctx, bin)
	if err != nil {
		t.Fatalf("wazero instantiate: %v", err)
	}
	out := make([]osdResult, len(cases))
	for i, c := range cases {
		fn := mod.ExportedFunction(c.fn)
		if fn == nil {
			t.Fatalf("%s not exported", c.fn)
		}
		args := make([]uint64, len(c.args))
		for j, a := range c.args {
			args[j] = api.EncodeI32(a)
		}
		res, err := fn.Call(ctx, args...)
		if err != nil {
			if !strings.Contains(err.Error(), "wasm error:") {
				t.Fatalf("wazero %s%v: %v", c.fn, c.args, err)
			}
			out[i] = osdResult{trap: true}
			continue
		}
		out[i] = osdResult{value: api.DecodeI32(res[0])}
	}
	return out
}

// osdTestClass renders the ABAP Unit local test class (ABAP 7.02) for a
// generated class: one test method per case. A trapping case asserts that
// the call raises; replayed trapping calls are caught.
func osdTestClass(class string, cases []osdCase, want []osdResult) string {
	return osdTestClassReplay(class, cases, want, true)
}

// moduleHasState reports whether a call can leave state behind for the next
// call (memory stores, memory.grow/fill/copy/init, global.set). Only then must
// each ABAP Unit method replay the preceding calls on its fresh instance;
// without state the replay only makes the test class grow quadratically.
func moduleHasState(mod *Module) bool {
	for _, f := range mod.Functions {
		for _, in := range f.Code {
			switch {
			case in.Op >= OpI32Store && in.Op <= OpI64Store32, in.Op == OpGlobalSet, in.Op == OpMemoryGrow:
				return true
			case in.Op == OpMiscPrefix && (in.MiscOp == MiscMemoryCopy || in.MiscOp == MiscMemoryFill || in.MiscOp == 0x08):
				return true
			}
		}
	}
	return false
}

func osdTestClassReplay(class string, cases []osdCase, want []osdResult, replay bool) string {
	var sb strings.Builder
	sb.WriteString("CLASS ltcl_wasm DEFINITION FINAL FOR TESTING\n")
	sb.WriteString("  DURATION SHORT RISK LEVEL HARMLESS.\n")
	sb.WriteString("  PRIVATE SECTION.\n")
	for i := range cases {
		fmt.Fprintf(&sb, "    METHODS c%d FOR TESTING.\n", i+1)
	}
	sb.WriteString("ENDCLASS.\n\n")
	sb.WriteString("CLASS ltcl_wasm IMPLEMENTATION.\n")
	for i, c := range cases {
		params := make([]string, len(c.args))
		shown := make([]string, len(c.args))
		for j, a := range c.args {
			params[j] = fmt.Sprintf("p%d = %d", j, a)
			shown[j] = fmt.Sprint(a)
		}
		label := fmt.Sprintf("%s(%s)", c.fn, strings.Join(shown, ","))
		fmt.Fprintf(&sb, "  METHOD c%d.\n", i+1)
		fmt.Fprintf(&sb, "    DATA lo TYPE REF TO %s.\n", class)
		sb.WriteString("    DATA lv_act TYPE i.\n")
		sb.WriteString("    DATA lv_exp TYPE i.\n")
		sb.WriteString("    DATA lv_trapped TYPE abap_bool.\n")
		sb.WriteString("    CREATE OBJECT lo.\n")
		// Replay preceding calls because each ABAP Unit method has a fresh instance.
		prev := cases[:i]
		if !replay {
			prev = nil
		}
		for j, prior := range prev {
			priorParams := make([]string, len(prior.args))
			for k, a := range prior.args {
				priorParams[k] = fmt.Sprintf("p%d = %d", k, a)
			}
			call := fmt.Sprintf("lv_act = lo->%s( %s ).", prior.fn, strings.Join(priorParams, " "))
			if want[j].trap {
				fmt.Fprintf(&sb, "    TRY.\n        %s\n      CATCH cx_root.\n    ENDTRY.\n", call)
			} else {
				fmt.Fprintf(&sb, "    %s\n", call)
			}
		}
		call := fmt.Sprintf("lv_act = lo->%s( %s ).", c.fn, strings.Join(params, " "))
		if want[i].trap {
			sb.WriteString("    lv_trapped = abap_false.\n")
			fmt.Fprintf(&sb, "    TRY.\n        %s\n      CATCH cx_root.\n        lv_trapped = abap_true.\n    ENDTRY.\n", call)
			fmt.Fprintf(&sb, "    cl_abap_unit_assert=>assert_true( act = lv_trapped msg = '%s must trap' ).\n", label)
		} else {
			fmt.Fprintf(&sb, "    %s\n", call)
			fmt.Fprintf(&sb, "    lv_exp = %d.\n", want[i].value)
			fmt.Fprintf(&sb, "    cl_abap_unit_assert=>assert_equals( act = lv_act exp = lv_exp msg = '%s' ).\n", label)
		}
		sb.WriteString("  ENDMETHOD.\n")
	}
	sb.WriteString("ENDCLASS.\n")
	return sb.String()
}

// TestOSD_EmitUnitClasses compiles the M1 modules and writes, per module, the
// class and its ABAP Unit test class in abapGit file layout. The OSD workflow
// sets ABAPITI_TEST_OUT, deploys the files and runs the tests.
func TestOSD_EmitUnitClasses(t *testing.T) {
	dir := testOutDir(t)
	emitWASIUnitClasses(t, dir)
	for _, m := range osdModules {
		var bin []byte
		var err error
		switch m.file {
		case "i32wrap":
			bin = buildI32WrapModule()
		case "branches":
			bin = buildBranchLoopWasm()
		case "suite":
			bin = buildSuiteWasm()
		case "memgrow":
			bin = buildMemoryModule(1, true)
		case "memzero":
			bin = buildMemoryModule(0, false)
		case "corpus":
			bin = compileCFixture(t, "../llvm/testdata/corpus.c")
		case "callind":
			bin = buildCallIndirectModule()
		case "floattrap":
			bin, _ = buildSpecialFloatModule()
		case "truncsat":
			bin, _, _ = buildTruncSatModule(false)
		case "i64wrap":
			bin = buildI64WrapModule(i64WrapCases)
		case "helpers":
			bin = buildHelperModule()
		default:
			bin, err = os.ReadFile(filepath.Join("testdata", m.file))
			if err != nil {
				t.Fatalf("read %s: %v", m.file, err)
			}
		}
		mod, err := Parse(bin)
		if err != nil {
			t.Fatalf("parse %s: %v", m.file, err)
		}
		want := wazeroResults(t, bin, m.cases)
		if m.file == "floattrap" {
			// WASM supports NaN/Inf (NaN trunc_sat returns zero), but ABAP
			// traps at their construction, before the conversion can run.
			for i := range want {
				want[i] = osdResult{trap: true}
			}
		}
		src := mustCompile(t, mod, m.class)
		tests := osdTestClassReplay(m.class, m.cases, want, moduleHasState(mod))
		checkTestClass(t, m.class, tests, want)
		for name, body := range map[string]string{
			m.class + ".clas.abap":             src,
			m.class + ".clas.testclasses.abap": tests,
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("%s: %d cases, expected %v", m.class, len(m.cases), want)
	}
}

var (
	reTestMethod = regexp.MustCompile(`(?m)^    METHODS c\d+ FOR TESTING\.$`)
	reExpected   = regexp.MustCompile(`(?m)^    lv_exp = (-?\d+)\.$`)
	reAssert     = regexp.MustCompile(`(?m)^    cl_abap_unit_assert=>assert_equals\( act = lv_act exp = lv_exp `)
	reTrapAssert = regexp.MustCompile(`(?m)^    cl_abap_unit_assert=>assert_true\( act = lv_trapped msg = '[^']*must trap' \)\.$`)
)

// checkTestClass reads the generated test class back and fails unless it has
// one test method per case, one expected literal and one assertion per value
// case, one trap assertion per trapping case, and the literals are the wazero
// results in order. It keeps the generated tests from becoming vacuous (an
// expected value copied from the actual one, or no methods at all), which OSD
// alone would report as green.
func checkTestClass(t *testing.T, class, src string, want []osdResult) {
	t.Helper()
	if len(want) == 0 {
		t.Fatalf("%s: no cases", class)
	}
	var values []int32
	traps := 0
	for _, w := range want {
		if w.trap {
			traps++
		} else {
			values = append(values, w.value)
		}
	}
	if n := len(reTestMethod.FindAllString(src, -1)); n != len(want) {
		t.Fatalf("%s: %d test methods, want %d", class, n, len(want))
	}
	if n := len(reAssert.FindAllString(src, -1)); n != len(values) {
		t.Fatalf("%s: %d assertions, want %d", class, n, len(values))
	}
	if n := len(reTrapAssert.FindAllString(src, -1)); n != traps {
		t.Fatalf("%s: %d trap assertions, want %d", class, n, traps)
	}
	got := reExpected.FindAllStringSubmatch(src, -1)
	if len(got) != len(values) {
		t.Fatalf("%s: %d expected literals, want %d", class, len(got), len(values))
	}
	for i, g := range got {
		v, err := strconv.ParseInt(g[1], 10, 32)
		if err != nil || int32(v) != values[i] {
			t.Fatalf("%s: value case %d expects %s, wazero says %d", class, i+1, g[1], values[i])
		}
	}
}
