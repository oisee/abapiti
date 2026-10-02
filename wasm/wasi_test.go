package wasm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// No toolchain or WASI sysroot is needed for this fixture. Each wrapper calls
// an actual preview1 import; peek lets ABAP Unit inspect the resulting layout.
func buildWASIModule() []byte {
	w := newWasmBuilder()
	types := []FuncType{
		{Params: []ValType{ValI32, ValI32, ValI32, ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI64, ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32}},
		{Results: []ValType{ValI32}},
		{Params: []ValType{ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI64, ValI32, ValI32}, Results: []ValType{ValI32}},
		{Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI32, ValI32}, Results: []ValType{ValI32}},
		{Params: []ValType{ValI32, ValI64, ValI64, ValI32}, Results: []ValType{ValI32}},
	}
	w.addSection(1, buildTypeSection(types))
	names := []string{"fd_write", "fd_read", "args_sizes_get", "args_get", "environ_sizes_get", "environ_get", "clock_time_get", "random_get", "proc_exit", "fd_fdstat_get", "fd_seek", "fd_prestat_get", "clock_res_get", "sched_yield", "poll_oneoff", "fd_close", "fd_prestat_dir_name", "fd_advise"}
	ti := []int{0, 0, 1, 1, 1, 1, 2, 1, 3, 1, 6, 1, 1, 7, 0, 5, 8, 9}
	imports := leb128u(uint32(len(names)))
	for i, n := range names {
		for _, s := range []string{"wasi_snapshot_preview1", n} {
			imports = append(imports, leb128u(uint32(len(s)))...)
			imports = append(imports, []byte(s)...)
		}
		imports = append(imports, 0)
		imports = append(imports, leb128u(uint32(ti[i]))...)
	}
	w.addSection(2, imports)
	var bodies [][]byte
	var funcs []int
	var exports []Export
	add := func(name string, typ int, code []byte) {
		exports = append(exports, Export{Name: name, Kind: 0, Index: len(names) + len(bodies)})
		funcs = append(funcs, typ)
		bodies = append(bodies, buildFuncBody(nil, code))
	}
	iconst := func(v int32) []byte { return append([]byte{OpI32Const}, leb128s(v)...) }
	call := func(index byte, args ...int32) []byte {
		var code []byte
		for _, a := range args {
			code = append(code, iconst(a)...)
		}
		return append(code, OpCall, index)
	}
	write := []byte{OpLocalGet, 0}
	for _, v := range []int32{0, 2, 120} {
		write = append(write, iconst(v)...)
	}
	write = append(write, OpCall, 0)
	add("write", 5, write)
	add("read", 4, call(1, 0, 0, 2, 124))
	add("read_bad", 4, call(1, 1, 0, 2, 124))
	add("args_sizes", 4, call(2, 128, 132))
	add("args_get", 4, call(3, 144, 256))
	add("env_sizes", 4, call(4, 136, 140))
	add("env_get", 4, call(5, 160, 320))
	for i, name := range []string{"realtime", "monotonic", "badclock"} {
		code := iconst(int32(i))
		code = append(code, OpI64Const, 0)
		code = append(code, iconst(512)...)
		code = append(code, OpCall, 6)
		add(name, 4, code)
	}
	add("random", 4, call(7, 400, 8))
	// Calling proc_exit from a nested function must unwind the exported caller.
	exitIndex := len(names) + len(bodies)
	exit := call(8, 23)
	exit = append(exit, iconst(99)...)
	add("exit_inner", 4, exit)
	add("exit_nested", 4, append([]byte{OpCall, byte(exitIndex), OpDrop}, iconst(99)...))
	stat := []byte{OpLocalGet, 0}
	stat = append(stat, iconst(544)...)
	stat = append(stat, OpCall, 9)
	add("stat", 5, stat)
	seek := []byte{OpLocalGet, 0, OpI64Const, 0}
	seek = append(seek, iconst(0)...)
	seek = append(seek, iconst(576)...)
	seek = append(seek, OpCall, 10)
	add("seek", 5, seek)
	add("prestat", 4, call(11, 3, 600))
	add("resolution", 4, call(12, 0, 520))
	add("yield", 4, call(13))
	add("poll", 4, call(14, 0, 0, 0, 0))
	add("close", 4, call(15, 1))
	add("prestat_name", 4, call(16, 3, 0, 0))
	add("peek", 5, []byte{OpLocalGet, 0, OpI32Load, 2, 0})
	advise := iconst(1)
	for range 2 {
		advise = append(advise, OpI64Const)
		advise = append(advise, leb128s64(1<<40)...)
	}
	advise = append(advise, iconst(0)...)
	advise = append(advise, OpCall, 17)
	add("unsupported", 4, advise)
	w.addSection(3, buildFuncSection(funcs))
	w.addSection(5, []byte{1, 0, 1})
	exports = append(exports, Export{Name: "memory", Kind: 2, Index: 0})
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	data := make([]byte, 600)
	// Dirty destinations make terminator, padding and flags checks meaningful.
	for i := 256; i < 384; i++ {
		data[i] = 0xff
	}
	for i := 544; i < 568; i++ {
		data[i] = 0xff
	}
	binary.LittleEndian.PutUint32(data[0:], 64)
	binary.LittleEndian.PutUint32(data[4:], 6)
	binary.LittleEndian.PutUint32(data[8:], 70)
	binary.LittleEndian.PutUint32(data[12:], 5)
	copy(data[64:], "hello world")
	section := []byte{1, 0, OpI32Const, 0, OpEnd}
	section = append(section, leb128u(uint32(len(data)))...)
	section = append(section, data...)
	w.addSection(11, section)
	return w.bytes()
}

// Emit these first so OSD still gets WASI tests when the optional C corpus
// skips generation on hosts without clang.
func emitWASIUnitClasses(t *testing.T, dir string) {
	t.Helper()
	const class = "zcl_abapiti_wasi"
	bin := buildWASIModule()
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	// Fixed bytes test the ABAP deterministic generator independently of wazero's
	// random source. These are the first eight high bytes of the documented LCG.
	random := []byte{0x3c, 0x5e, 0x81, 0xb4, 0x0c, 0x5e, 0xc6, 0x8e}
	wm, err := rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().WithArgs(class).WithStdout(&stdout).WithStderr(&stderr).WithStdin(strings.NewReader("abc\x00xyz")).WithRandSource(bytes.NewReader(random)).WithWalltime(func() (int64, int32) { return 1700000000, 123456789 }, sys.ClockResolution(1000)).WithNanotime(func() int64 { return 123456789 }, sys.ClockResolution(1000)))
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString("CLASS ltcl_wasm DEFINITION FINAL FOR TESTING\n  DURATION SHORT RISK LEVEL HARMLESS.\n  PRIVATE SECTION.\n    METHODS c1 FOR TESTING.\nENDCLASS.\nCLASS ltcl_wasm IMPLEMENTATION.\n  METHOD c1.\n")
	fmt.Fprintf(&sb, "    DATA lo TYPE REF TO %s.\n", class)
	sb.WriteString("    DATA lv_act TYPE i.\n    DATA lv_bytes TYPE xstring.\n    DATA lv_exp_bytes TYPE xstring.\n    DATA lv_trapped TYPE abap_bool.\n    DATA lt_strings TYPE string_table.\n    CREATE OBJECT lo.\n")
	invoke := func(name string, args ...uint64) uint64 {
		res, e := wm.ExportedFunction(name).Call(ctx, args...)
		if e != nil {
			t.Fatalf("wazero %s: %v", name, e)
		}
		if len(res) != 1 {
			t.Fatal("missing result")
		}
		return res[0]
	}
	assertI := func(expr string, want int32) {
		fmt.Fprintf(&sb, "    lv_act = %s.\n    cl_abap_unit_assert=>assert_equals( act = lv_act exp = %d ).\n", expr, want)
	}
	run := func(name string, args ...uint64) {
		param := ""
		if len(args) > 0 {
			param = fmt.Sprintf("p0 = %d", args[0])
		}
		assertI(fmt.Sprintf("lo->%s( %s )", name, param), int32(invoke(name, args...)))
	}
	peek := func(addr uint32) {
		v, ok := wm.Memory().ReadUint32Le(addr)
		if !ok {
			t.Fatal("memory read")
		}
		assertI(fmt.Sprintf("lo->peek( p0 = %d )", addr), int32(v))
	}
	assertBytes := func(expr string, want []byte) {
		fmt.Fprintf(&sb, "    lv_bytes = %s.\n    lv_exp_bytes = '%X'.\n    cl_abap_unit_assert=>assert_equals( act = lv_bytes exp = lv_exp_bytes ).\n", expr, want)
	}
	run("write", 1)
	run("write", 1)
	assertBytes("lo->get_stdout( )", stdout.Bytes())
	peek(120)
	run("write", 2)
	assertBytes("lo->get_stderr( )", stderr.Bytes())
	run("write", 3)
	run("args_sizes")
	peek(128)
	peek(132)
	run("args_get")
	peek(144)
	peek(256)
	peek(260)
	peek(268)
	peek(272)
	run("env_sizes")
	peek(136)
	peek(140)
	run("env_get")
	// Configured arguments exercise multiple pointers, empty strings, UTF-8 and NULs.
	sb.WriteString("    APPEND 'x' TO lt_strings.\n    APPEND '' TO lt_strings.\n    APPEND 'é' TO lt_strings.\n    lo->set_args( lt_strings ).\n")
	assertI("lo->args_sizes( )", 0)
	assertI("lo->peek( p0 = 128 )", 3)
	assertI("lo->peek( p0 = 132 )", 6)
	assertI("lo->args_get( )", 0)
	assertI("lo->peek( p0 = 144 )", 256)
	assertI("lo->peek( p0 = 148 )", 258)
	assertI("lo->peek( p0 = 152 )", 259)
	assertI("lo->peek( p0 = 256 )", -1023410056)
	sb.WriteString("    CLEAR lt_strings.\n    APPEND 'A=B' TO lt_strings.\n    lo->set_env( lt_strings ).\n")
	assertI("lo->env_sizes( )", 0)
	assertI("lo->peek( p0 = 136 )", 1)
	assertI("lo->peek( p0 = 140 )", 4)
	assertI("lo->env_get( )", 0)
	assertI("lo->peek( p0 = 160 )", 320)
	assertI("lo->peek( p0 = 320 )", 0x423d41)
	sb.WriteString("    lv_bytes = '6162630078797A'.\n    lo->set_stdin( lv_bytes ).\n")
	run("read_bad")
	run("read")
	peek(124)
	peek(64)
	peek(70)
	run("read")
	peek(124)
	// Time is checked against epoch bounds rather than compared to a generation-time clock.
	invoke("realtime")
	ns, ok := wm.Memory().ReadUint64Le(512)
	if !ok || ns != 1700000000123456789 {
		t.Fatalf("reference realtime: %d", ns)
	}
	assertI("lo->realtime( )", 0)
	sb.WriteString("    lv_act = lo->peek( p0 = 516 ).\n    lv_trapped = abap_false.\n    IF lv_act > 367368936 AND lv_act < 955187321.\n      lv_trapped = abap_true.\n    ENDIF.\n    cl_abap_unit_assert=>assert_true( act = lv_trapped ).\n    lv_trapped = abap_false.\n")
	assertI("lo->monotonic( )", 0)
	run("badclock")
	run("resolution")
	peek(520)
	peek(524)
	run("random")
	peek(400)
	peek(404)
	for _, fd := range []uint64{0, 1, 2} {
		run("stat", fd)
		assertI("lo->peek( p0 = 544 )", 2)
		assertI("lo->peek( p0 = 548 )", 0)
		for _, addr := range []int{552, 556, 560, 564} {
			assertI(fmt.Sprintf("lo->peek( p0 = %d )", addr), -1)
		}
		assertI(fmt.Sprintf("lo->seek( p0 = %d )", fd), 70)
	}
	run("stat", 3)
	run("prestat")
	run("prestat_name")
	run("yield")
	run("close")
	assertI("lo->poll( )", 52)
	assertI("lo->unsupported( )", 52)
	assertI("lo->seek( p0 = 3 )", 8)
	assertI("lo->get_exit_code( )", -1)
	_, err = wm.ExportedFunction("exit_nested").Call(ctx)
	exitErr, ok := err.(*sys.ExitError)
	if !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("reference exit: %v", err)
	}
	sb.WriteString("    TRY.\n        lv_act = lo->exit_nested( ).\n      CATCH cx_sy_dyn_call_illegal_method.\n        lv_trapped = abap_true.\n    ENDTRY.\n    cl_abap_unit_assert=>assert_true( act = lv_trapped ).\n")
	assertI("lo->get_exit_code( )", 23)
	sb.WriteString("  ENDMETHOD.\nENDCLASS.\n")
	for name, src := range map[string]string{class + ".clas.abap": Compile(mod, class), class + ".clas.testclasses.abap": sb.String()} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWASIGeneration(t *testing.T) {
	mod, err := Parse(buildWASIModule())
	if err != nil {
		t.Fatal(err)
	}
	multi := CompileMultiClass(mod, "zcl_wasi", 80)
	for name, src := range map[string]string{"class": Compile(mod, "zcl_wasi"), "state": multi.MainClass} {
		assertNoABAPComments(t, name, src)
		for _, bad := range []string{"PERFORM ", "DATA(", "CONV ", "NEW ", "zcl_wasm_rt=>", "zcl_wasm_wasi", "iv_name = 'fd_advise'"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %s", name, bad)
			}
		}
		for _, required := range []string{"METHOD get_stdout.", "METHOD set_stdin.", "METHOD set_args.", "METHOD set_env.", "METHOD mem_st_i64.", "mv_exit_code = p0.", wasmTrap, "rv = 52."} {
			if !strings.Contains(src, required) {
				t.Errorf("%s missing %s", name, required)
			}
		}
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Errorf("%s line length %d", name, len(line))
			}
		}
	}
	for _, src := range multi.ChunkClasses {
		for _, required := range []string{"mo_main->wasi_call(", "mo_main->mem_ld_i32(", "s1_i64 TYPE int8"} {
			if !strings.Contains(src, required) {
				t.Errorf("chunk missing %s", required)
			}
		}
		if parameterWrite.MatchString(src) {
			t.Error("chunk writes to IMPORTING parameter")
		}
	}

	for _, backend := range []BackendKind{BackendFUGR, BackendHybrid} {
		result := CompileWith(mod, "zwasi", backend, 80)
		for name, src := range result.Files {
			assertNoABAPComments(t, name, src)
			if strings.Contains(src, "wasi_call(") {
				t.Errorf("%s invalid FUGR WASI method call", name)
			}
		}
	}
	emitWASIUnitClasses(t, t.TempDir())
}
