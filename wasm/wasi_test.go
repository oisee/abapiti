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
	"time"

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
	add("close", 5, []byte{OpLocalGet, 0, OpCall, 15})
	add("trap", 4, []byte{OpUnreachable, OpI32Const, 0})
	add("raw_write", 0, []byte{OpLocalGet, 0, OpLocalGet, 1, OpLocalGet, 2, OpLocalGet, 3, OpCall, 0})
	add("raw_read", 0, []byte{OpLocalGet, 0, OpLocalGet, 1, OpLocalGet, 2, OpLocalGet, 3, OpCall, 1})
	add("poke", 1, []byte{OpLocalGet, 0, OpLocalGet, 1, OpI32Store, 2, 0, OpI32Const, 0})
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
	for _, spec := range []struct {
		name   string
		typ    int
		imp    byte
		params int
	}{
		{"raw_stat", 1, 9, 2}, {"raw_random", 1, 7, 2},
		{"raw_sizes", 1, 2, 2}, {"raw_args", 1, 3, 2},
		{"raw_time", 2, 6, 3}, {"raw_res", 1, 12, 2},
	} {
		var code []byte
		for i := 0; i < spec.params; i++ {
			code = append(code, OpLocalGet, byte(i))
		}
		add(spec.name, spec.typ, append(code, OpCall, spec.imp))
	}
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
	wallSeconds := int64(1700000000)
	monotonicNS := int64(123456789)
	wm, err := rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().WithArgs(class).WithStdout(&stdout).WithStderr(&stderr).WithStdin(strings.NewReader("abc\x00xyz")).WithRandSource(bytes.NewReader(random)).WithWalltime(func() (int64, int32) { return wallSeconds, 0 }, sys.ClockResolution(1000)).WithNanotime(func() int64 { return monotonicNS }, sys.ClockResolution(1000)))
	if err != nil {
		t.Fatal(err)
	}
	// Buffer-backed wazero streams report block devices. Use actual character
	// devices for the fdstat oracle to match the generated stream abstraction.
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	statModule, err := rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().WithName("stat_oracle").WithStdin(devnull).WithStdout(devnull).WithStderr(devnull))
	if err != nil {
		t.Fatal(err)
	}
	defer statModule.Close(ctx)
	var fdstats [3][24]byte
	for fd := range fdstats {
		result, err := statModule.ExportedFunction("stat").Call(ctx, uint64(fd))
		if err != nil || len(result) != 1 || result[0] != 0 {
			t.Fatalf("reference fdstat %d: %v, %v", fd, result, err)
		}
		stat, ok := statModule.Memory().Read(544, 24)
		if !ok {
			t.Fatal("reference fdstat memory")
		}
		copy(fdstats[fd][:], stat)
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
		params := make([]string, len(args))
		for i, arg := range args {
			params[i] = fmt.Sprintf("p%d = %d", i, int32(arg))
		}
		assertI(fmt.Sprintf("lo->%s( %s )", name, strings.Join(params, " ")), int32(invoke(name, args...)))
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
	// Whole-second packed timestamps exercise the integer calendar path; wazero
	// independently supplies epoch nanoseconds for both words, including leap days.
	for _, instant := range []string{
		"1970-01-01T00:00:00Z", "2000-02-29T23:59:59Z",
		"2023-11-14T22:13:20Z", "2026-10-02T12:34:56Z",
		"2100-03-01T00:00:00Z",
	} {
		fixed, err := time.Parse(time.RFC3339, instant)
		if err != nil {
			t.Fatal(err)
		}
		wallSeconds = fixed.Unix()
		fmt.Fprintf(&sb, "    lo->mv_clock_override_ts = %s.\n", fixed.Format("20060102150405"))
		run("realtime")
		peek(512)
		peek(516)
	}
	sb.WriteString("    lo->mv_clock_override_ts = -1.\n")
	assertI("lo->realtime( )", 0)
	// Cover the signed boundary, repeated samples, wrap to zero, and a second
	// wrap. The seam substitutes only the raw counter, preserving state logic.
	for _, sample := range []struct{ raw, extended int64 }{
		{2147483647, 2147483647}, {2147483648, 2147483648},
		{4294967295, 4294967295}, {0, 4294967296},
		{0, 4294967296}, {1, 4294967297},
		{4294967295, 8589934591}, {0, 8589934592},
	} {
		fmt.Fprintf(&sb, "    lo->mv_runtime_override_us = %d.\n", sample.raw)
		monotonicNS = sample.extended * 1000
		run("monotonic")
		peek(512)
		peek(516)
	}
	sb.WriteString("    lo->mv_runtime_override_us = -1.\n")
	assertI("lo->monotonic( )", 0)
	run("badclock")
	// Wazero rejects one-second resolution, so verify our coarser realtime
	// contract directly; the monotonic resolution still uses the oracle.
	assertI("lo->resolution( )", 0)
	assertI("lo->peek( p0 = 520 )", 1000000000)
	assertI("lo->peek( p0 = 524 )", 0)
	run("raw_res", 1, 520)
	peek(520)
	peek(524)
	run("random")
	peek(400)
	peek(404)
	for _, fd := range []uint64{0, 1, 2} {
		run("stat", fd)
		// Six words compare the entire 24-byte layout with wazero for each fd,
		// including padding, flags, base rights and zero inheriting rights.
		for offset := 0; offset < 24; offset += 4 {
			want := binary.LittleEndian.Uint32(fdstats[fd][offset:])
			assertI(fmt.Sprintf("lo->peek( p0 = %d )", 544+offset), int32(want))
		}
		assertI(fmt.Sprintf("lo->seek( p0 = %d )", fd), 70)
	}
	run("stat", 3)
	run("prestat")
	run("prestat_name")
	run("yield")
	// Invalid unsigned iovecs, buffers and result pointers return EFAULT rather
	// than trapping. Run every case through both read and write on wazero.
	for _, name := range []string{"raw_write", "raw_read"} {
		fd := uint64(1)
		if name == "raw_read" {
			fd = 0
		}
		for _, args := range [][]uint64{
			{fd, 65532, 1, 120}, {fd, 0xffffffff, 1, 120},
			{fd, 0, 0xffffffff, 120}, {fd, 0, 1, 65533},
			{fd, 0, 1, 0xffffffff}, {fd, 65536, 0, 120},
		} {
			run(name, args...)
		}
		for _, iov := range [][2]uint64{{65535, 2}, {0xffffffff, 1}, {64, 0xffffffff}, {65536, 0}} {
			run("poke", 0, iov[0])
			run("poke", 4, iov[1])
			run(name, fd, 0, 1, 120)
		}
	}
	for _, tc := range []struct {
		name string
		args []uint64
	}{
		{"raw_stat", []uint64{1, 65513}}, {"raw_stat", []uint64{1, 0xffffffff}},
		{"raw_random", []uint64{65535, 2}}, {"raw_random", []uint64{0, 0xffffffff}},
		{"raw_sizes", []uint64{65533, 132}}, {"raw_sizes", []uint64{128, 0xffffffff}},
		{"raw_args", []uint64{65533, 256}}, {"raw_args", []uint64{144, 65535}},
		{"raw_time", []uint64{0, 0, 65529}}, {"raw_res", []uint64{0, 0xffffffff}},
	} {
		run(tc.name, tc.args...)
	}
	run("poke", 0, 64)
	run("poke", 4, 6)
	for _, fd := range []uint64{99, 0xffffffff} {
		run("close", fd)
	}
	for _, fd := range []uint64{1, 0, 2} {
		run("close", fd)
		run("close", fd)
		run("stat", fd)
		run("seek", fd)
		if fd == 0 {
			run("read")
		} else {
			run("write", fd)
		}
	}
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
	sb.WriteString("    cl_abap_unit_assert=>assert_true( act = lo->mv_exited ).\n    lv_trapped = abap_false.\n    TRY.\n        lv_act = lo->trap( ).\n      CATCH cx_sy_dyn_call_illegal_method.\n        lv_trapped = abap_true.\n    ENDTRY.\n    cl_abap_unit_assert=>assert_true( act = lv_trapped ).\n")
	assertI("lo->get_exit_code( )", -1)
	sb.WriteString("    cl_abap_unit_assert=>assert_false( act = lo->mv_exited ).\n")
	assertI("lo->yield( )", 0)
	assertI("lo->get_exit_code( )", -1)
	sb.WriteString("  ENDMETHOD.\nENDCLASS.\n")
	for name, src := range map[string]string{class + ".clas.abap": mustCompile(t, mod, class), class + ".clas.testclasses.abap": sb.String()} {
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
	multi, err := CompileMultiClass(mod, "zcl_wasi", 80)
	if err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{"class": mustCompile(t, mod, "zcl_wasi"), "state": multi.StateClass} {
		assertNoABAPComments(t, name, src)
		if parameterWrite.MatchString(src) {
			t.Errorf("%s writes to IMPORTING parameter", name)
		}
		for _, bad := range []string{"PERFORM ", "DATA(", "CONV ", "NEW ", "zcl_wasm_rt=>", "zcl_wasm_wasi", "iv_name = 'fd_advise'"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %s", name, bad)
			}
		}
		for _, required := range []string{"METHOD get_stdout.", "METHOD set_stdin.", "METHOD set_args.", "METHOD set_env.", "METHOD mem_st_i64.", "mv_exit_code = p0.", wasmTrap, "rv = 52.", "mv_exited = abap_true.", "mv_exited = abap_false."} {
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
	for name, src := range multi.ChunkClasses {
		assertNoABAPComments(t, name, src)
		if strings.Contains(src, "mv_exit_code = -1.") || strings.Contains(src, "mv_exited = abap_false.") {
			t.Error("chunk resets exit state during internal calls")
		}
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Errorf("%s line length %d", name, len(line))
			}
		}
		for _, required := range []string{multi.StateName + "=>wasi_call(", "s1_i64 TYPE int8"} {
			if !strings.Contains(src, required) {
				t.Errorf("chunk missing %s", required)
			}
		}
		if parameterWrite.MatchString(src) {
			t.Error("chunk writes to IMPORTING parameter")
		}
	}

	chunks := ""
	for _, src := range multi.ChunkClasses {
		chunks += src
	}
	if !strings.Contains(chunks, multi.StateName+"=>mem_ld_i32(") {
		t.Error("missing chunk memory access")
	}
	for _, api := range []string{"get_stdout", "get_stderr", "get_exit_code", "set_stdin", "set_args", "set_env"} {
		if !strings.Contains(multi.MainClass, "METHOD "+api+".") || !strings.Contains(multi.MainClass, multi.StateName+"=>"+api+"(") {
			t.Errorf("facade does not expose %s", api)
		}
	}
	if strings.Count(multi.MainClass, multi.StateName+"=>wasi_reset( ).") != len(splitExports(mod)) {
		t.Error("facade must reset exit status once per export")
	}

	for _, backend := range []BackendKind{BackendFUGR, BackendHybrid} {
		result := mustCompileWith(t, mod, "zwasi", backend, 80)
		for name, src := range result.Files {
			assertNoABAPComments(t, name, src)
			if strings.Contains(src, "wasi_call(") {
				t.Errorf("%s invalid FUGR WASI method call", name)
			}
		}
	}
	emitWASIUnitClasses(t, t.TempDir())
}
