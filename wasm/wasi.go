package wasm

import (
	"fmt"
	"strings"
)

func (c *compiler) hasWASI() bool {
	for _, imp := range c.mod.Imports {
		if imp.Kind == 0 && imp.Module == "wasi_snapshot_preview1" {
			return true
		}
	}
	return false
}

func (c *compiler) emitWASIDeclarations() {
	if !c.hasWASI() {
		return
	}
	for _, line := range strings.Split(`DATA mv_stdout TYPE xstring.
DATA mv_stderr TYPE xstring.
DATA mv_stdin TYPE xstring.
DATA mv_stdin_pos TYPE i.
DATA mv_exit_code TYPE i.
DATA mv_exited TYPE abap_bool.
DATA mv_closed0 TYPE abap_bool.
DATA mv_closed1 TYPE abap_bool.
DATA mv_closed2 TYPE abap_bool.
DATA mv_clock_override_ts TYPE int8 VALUE -1.
DATA mv_runtime_override_us TYPE int8 VALUE -1.
DATA mv_clock_last_us TYPE int8.
DATA mv_clock_wrap_us TYPE int8.
DATA mv_random TYPE int8.
DATA mt_args TYPE string_table.
DATA mt_env TYPE string_table.
METHODS get_stdout RETURNING VALUE(rv) TYPE xstring.
METHODS get_stderr RETURNING VALUE(rv) TYPE xstring.
METHODS set_stdin IMPORTING iv TYPE xstring.
METHODS get_exit_code RETURNING VALUE(rv) TYPE i.
METHODS set_args IMPORTING it TYPE string_table.
METHODS set_env IMPORTING it TYPE string_table.
METHODS wasi_range IMPORTING iv_ptr TYPE i VALUE(iv_len) TYPE int8 RETURNING VALUE(rv) TYPE abap_bool.
METHODS wasi_fd_open IMPORTING iv_fd TYPE i RETURNING VALUE(rv) TYPE abap_bool.
METHODS wasi_call IMPORTING iv_name TYPE string p0 TYPE i DEFAULT 0 p1 TYPE any OPTIONAL
p2 TYPE i DEFAULT 0 p3 TYPE i DEFAULT 0 RETURNING VALUE(rv) TYPE i.`, "\n") {
		c.line("%s", line)
	}
}

func (c *compiler) emitWASIInit() {
	if !c.hasWASI() {
		return
	}
	c.line("APPEND '%s' TO mt_args.", strings.ReplaceAll(c.className, "'", "''"))
	c.line("mv_random = 1.")
	// -1 distinguishes an ordinary WASM trap from proc_exit(0).
	c.line("mv_exit_code = -1.")
}

func (c *compiler) emitWASIImplementation() {
	if !c.hasWASI() {
		return
	}
	// Realtime has one-second resolution: timestamp's 14 integral digits are
	// exact even on double-backed hosts. Copy to int8 before DIV/MOD, then use
	// Gregorian civil-day arithmetic and int8 epoch seconds/nanoseconds.
	// Monotonic extends the unsigned 32-bit microsecond counter across rollovers.
	// Wazero advertises the same nonseekable character-device rights for each
	// of stdin/stdout/stderr: file rights minus SEEK/TELL, inheriting rights zero.
	// Its character-device stdio wrappers also advertise the APPEND flag.
	// This reproducible LCG is deliberately not cryptographic randomness.
	body := `METHOD get_stdout.
rv = mv_stdout.
ENDMETHOD.
METHOD get_stderr.
rv = mv_stderr.
ENDMETHOD.
METHOD get_exit_code.
rv = mv_exit_code.
ENDMETHOD.
METHOD set_stdin.
mv_stdin = iv.
mv_stdin_pos = 0.
ENDMETHOD.
METHOD set_args.
mt_args = it.
ENDMETHOD.
METHOD set_env.
mt_env = it.
ENDMETHOD.
METHOD wasi_range.
DATA lv_ptr TYPE int8.
DATA lv_len TYPE int8.
DATA lv_size TYPE int8.
lv_ptr = iv_ptr.
lv_len = iv_len.
IF lv_ptr < 0. lv_ptr = lv_ptr + 4294967296. ENDIF.
IF lv_len < 0. lv_len = lv_len + 4294967296. ENDIF.
lv_size = xstrlen( mv_mem ).
rv = abap_false.
IF lv_ptr >= 0 AND lv_len >= 0 AND lv_ptr + lv_len <= lv_size.
rv = abap_true.
ENDIF.
ENDMETHOD.
METHOD wasi_fd_open.
rv = abap_false.
CASE iv_fd.
WHEN 0. IF mv_closed0 = abap_false. rv = abap_true. ENDIF.
WHEN 1. IF mv_closed1 = abap_false. rv = abap_true. ENDIF.
WHEN 2. IF mv_closed2 = abap_false. rv = abap_true. ENDIF.
ENDCASE.
ENDMETHOD.
METHOD wasi_call.
DATA lv_addr TYPE i.
DATA lv_arg1 TYPE i.
DATA lv_ptr TYPE i.
DATA lv_len TYPE i.
DATA lv_total TYPE int8.
DATA lv_span TYPE int8.
DATA lv_count TYPE i.
DATA lv_buf TYPE i.
DATA lv_left TYPE i.
DATA lv_bytes TYPE xstring.
DATA lv_zero TYPE x LENGTH 1.
DATA lv_stat TYPE x LENGTH 24.
DATA lv_strings TYPE string_table.
DATA lv_string TYPE string.
DATA lo_utf8 TYPE REF TO cl_abap_conv_out_ce.
DATA lv_ts TYPE timestamp.
DATA lv_stamp TYPE int8.
DATA lv_year TYPE int8.
DATA lv_month TYPE int8.
DATA lv_day TYPE int8.
DATA lv_era TYPE int8.
DATA lv_yoe TYPE int8.
DATA lv_days TYPE int8.
DATA lv_hours TYPE int8.
DATA lv_minutes TYPE int8.
DATA lv_seconds TYPE int8.
DATA lv_ns TYPE int8.
DATA lv_us TYPE int8.
DATA lv_runtime TYPE i.
rv = 0.
CASE iv_name.
WHEN 'fd_write' OR 'fd_read' OR 'fd_fdstat_get' OR 'args_sizes_get' OR 'args_get'
OR 'environ_sizes_get' OR 'environ_get' OR 'clock_res_get' OR 'random_get'.
lv_arg1 = p1.
ENDCASE.
CASE iv_name.
WHEN 'fd_write' OR 'fd_read'.
IF wasi_fd_open( p0 ) = abap_false. rv = 8. RETURN. ENDIF.
IF iv_name = 'fd_write'.
IF p0 <> 1 AND p0 <> 2. rv = 8. RETURN. ENDIF.
ELSE.
IF p0 <> 0. rv = 8. RETURN. ENDIF.
ENDIF.
lv_span = p2.
IF lv_span < 0. lv_span = lv_span + 4294967296. ENDIF.
lv_span = lv_span * 8.
IF wasi_range( iv_ptr = lv_arg1 iv_len = lv_span ) = abap_false OR
wasi_range( iv_ptr = p3 iv_len = 4 ) = abap_false. rv = 21. RETURN. ENDIF.
lv_count = p2.
lv_addr = lv_arg1.
DO lv_count TIMES.
lv_ptr = mem_ld_i32( lv_addr ).
lv_len = mem_ld_i32( lv_addr + 4 ).
lv_span = lv_len.
IF wasi_range( iv_ptr = lv_ptr iv_len = lv_span ) = abap_false. rv = 21. RETURN. ENDIF.
IF iv_name = 'fd_write'.
IF lv_len > 0.
lv_bytes = mv_mem+lv_ptr(lv_len).
IF p0 = 1.
CONCATENATE mv_stdout lv_bytes INTO mv_stdout IN BYTE MODE.
ELSE.
CONCATENATE mv_stderr lv_bytes INTO mv_stderr IN BYTE MODE.
ENDIF.
ENDIF.
ELSE.
lv_left = xstrlen( mv_stdin ) - mv_stdin_pos.
IF lv_len > lv_left. lv_len = lv_left. ENDIF.
IF lv_len > 0.
lv_bytes = mv_stdin+mv_stdin_pos(lv_len).
REPLACE SECTION OFFSET lv_ptr LENGTH lv_len OF mv_mem WITH lv_bytes IN BYTE MODE.
mv_stdin_pos = mv_stdin_pos + lv_len.
ENDIF.
ENDIF.
lv_total = lv_total + lv_len.
lv_addr = lv_addr + 8.
ENDDO.
lv_total = lv_total MOD 4294967296.
IF lv_total > 2147483647. lv_total = lv_total - 4294967296. ENDIF.
lv_len = lv_total.
mem_st_i32( iv_addr = p3 iv_val = lv_len ).
WHEN 'fd_close'.
IF wasi_fd_open( p0 ) = abap_false. rv = 8. RETURN. ENDIF.
CASE p0.
WHEN 0. mv_closed0 = abap_true.
WHEN 1. mv_closed1 = abap_true.
WHEN 2. mv_closed2 = abap_true.
ENDCASE.
WHEN 'sched_yield'.
rv = 0.
WHEN 'fd_seek'.
IF wasi_fd_open( p0 ) = abap_true. rv = 70. ELSE. rv = 8. ENDIF.
WHEN 'fd_fdstat_get'.
IF wasi_fd_open( p0 ) = abap_false. rv = 8. RETURN. ENDIF.
IF wasi_range( iv_ptr = lv_arg1 iv_len = 24 ) = abap_false. rv = 21. RETURN. ENDIF.
lv_addr = lv_arg1.
REPLACE SECTION OFFSET lv_addr LENGTH 24 OF mv_mem WITH lv_stat IN BYTE MODE.
mem_st_i32_8( iv_addr = lv_addr iv_val = 2 ).
mem_st_i32_16( iv_addr = lv_addr + 2 iv_val = 1 ).
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = lv_addr + 8 iv_val = 148898267 CHANGING cv_mem = mv_mem ).
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = lv_addr + 16 iv_val = 0 CHANGING cv_mem = mv_mem ).
WHEN 'fd_prestat_get' OR 'fd_prestat_dir_name'.
rv = 8.
WHEN 'args_sizes_get' OR 'environ_sizes_get' OR 'args_get' OR 'environ_get'.
IF iv_name = 'args_sizes_get' OR iv_name = 'args_get'.
lv_strings = mt_args.
ELSE.
lv_strings = mt_env.
ENDIF.
IF iv_name = 'args_sizes_get' OR iv_name = 'environ_sizes_get'.
IF wasi_range( iv_ptr = p0 iv_len = 4 ) = abap_false OR
wasi_range( iv_ptr = lv_arg1 iv_len = 4 ) = abap_false. rv = 21. RETURN. ENDIF.
ELSE.
lv_count = lines( lv_strings ).
lv_span = lv_count.
lv_span = lv_span * 4.
IF wasi_range( iv_ptr = p0 iv_len = lv_span ) = abap_false. rv = 21. RETURN. ENDIF.
ENDIF.
lv_addr = p0.
lv_buf = lv_arg1.
LOOP AT lv_strings INTO lv_string.
lo_utf8 = cl_abap_conv_out_ce=>create( encoding = 'UTF-8' ).
lo_utf8->write( data = lv_string ).
lv_bytes = lo_utf8->get_buffer( ).
CONCATENATE lv_bytes lv_zero INTO lv_bytes IN BYTE MODE.
lv_len = xstrlen( lv_bytes ).
IF iv_name = 'args_get' OR iv_name = 'environ_get'.
lv_span = lv_len.
IF wasi_range( iv_ptr = lv_buf iv_len = lv_span ) = abap_false. rv = 21. RETURN. ENDIF.
mem_st_i32( iv_addr = lv_addr iv_val = lv_buf ).
REPLACE SECTION OFFSET lv_buf LENGTH lv_len OF mv_mem WITH lv_bytes IN BYTE MODE.
lv_addr = lv_addr + 4.
lv_buf = lv_buf + lv_len.
ENDIF.
lv_total = lv_total + lv_len.
ENDLOOP.
IF iv_name = 'args_sizes_get' OR iv_name = 'environ_sizes_get'.
lv_count = lines( lv_strings ).
mem_st_i32( iv_addr = p0 iv_val = lv_count ).
lv_addr = lv_arg1.
lv_len = lv_total.
mem_st_i32( iv_addr = lv_addr iv_val = lv_len ).
ENDIF.
WHEN 'clock_time_get'.
IF wasi_range( iv_ptr = p2 iv_len = 8 ) = abap_false. rv = 21. RETURN. ENDIF.
IF p0 = 0.
GET TIME STAMP FIELD lv_ts.
lv_stamp = lv_ts.
IF mv_clock_override_ts >= 0. lv_stamp = mv_clock_override_ts. ENDIF.
lv_year = lv_stamp DIV 10000000000.
lv_month = lv_stamp DIV 100000000 MOD 100.
lv_day = lv_stamp DIV 1000000 MOD 100.
lv_hours = lv_stamp DIV 10000 MOD 100.
lv_minutes = lv_stamp DIV 100 MOD 100.
lv_seconds = lv_stamp MOD 100.
IF lv_month <= 2.
lv_year = lv_year - 1.
lv_month = lv_month + 9.
ELSE.
lv_month = lv_month - 3.
ENDIF.
lv_era = lv_year DIV 400.
lv_yoe = lv_year MOD 400.
lv_days = lv_era * 146097 + lv_yoe * 365 + lv_yoe DIV 4 - lv_yoe DIV 100.
lv_days = lv_days + ( 153 * lv_month + 2 ) DIV 5 + lv_day - 1 - 719468.
lv_seconds = lv_days * 86400 + lv_hours * 3600 + lv_minutes * 60 + lv_seconds.
lv_ns = lv_seconds * 1000000000.
ELSEIF p0 = 1.
GET RUN TIME FIELD lv_runtime.
lv_us = lv_runtime.
IF mv_runtime_override_us >= 0. lv_us = mv_runtime_override_us. ENDIF.
IF lv_us < 0. lv_us = lv_us + 4294967296. ENDIF.
lv_us = lv_us + mv_clock_wrap_us.
IF lv_us < mv_clock_last_us.
mv_clock_wrap_us = mv_clock_wrap_us + 4294967296.
lv_us = lv_us + 4294967296.
ENDIF.
mv_clock_last_us = lv_us.
lv_ns = lv_us * 1000.
ELSE.
rv = 28. RETURN.
ENDIF.
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = p2 iv_val = lv_ns CHANGING cv_mem = mv_mem ).
WHEN 'clock_res_get'.
IF wasi_range( iv_ptr = lv_arg1 iv_len = 8 ) = abap_false. rv = 21. RETURN. ENDIF.
IF p0 <> 0 AND p0 <> 1. rv = 28. RETURN. ENDIF.
lv_addr = lv_arg1.
lv_ns = 1000.
IF p0 = 0. lv_ns = 1000000000. ENDIF.
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = lv_addr iv_val = lv_ns CHANGING cv_mem = mv_mem ).
WHEN 'random_get'.
lv_span = lv_arg1.
IF wasi_range( iv_ptr = p0 iv_len = lv_span ) = abap_false. rv = 21. RETURN. ENDIF.
lv_addr = p0.
lv_count = lv_arg1.
DO lv_count TIMES.
mv_random = ( mv_random * 1664525 + 1013904223 ) MOD 4294967296.
lv_len = mv_random DIV 16777216.
mem_st_i32_8( iv_addr = lv_addr iv_val = lv_len ).
lv_addr = lv_addr + 1.
ENDDO.
WHEN 'proc_exit'.
mv_exit_code = p0.
mv_exited = abap_true.
RAISE EXCEPTION TYPE cx_sy_dyn_call_illegal_method.
WHEN OTHERS.
rv = 52.
ENDCASE.
ENDMETHOD.`
	for _, line := range strings.Split(body, "\n") {
		c.line("%s", line)
	}
}

// Export facades reset status exactly once per host invocation. WASM calls,
// including calls to exported functions and indirect calls, use private bodies
// so they cannot erase proc_exit while unwinding. Multi-class facades reset the
// shared main state before entering a chunk.
func (c *compiler) wasiExportWrappers() bool { return c.hasWASI() && !c.sharedMain && !c.useFUGR }

func (c *compiler) emitWASIReset() {
	if c.hasWASI() {
		c.line("mv_exit_code = -1.")
		c.line("mv_exited = abap_false.")
	}
}

func (c *compiler) emitWASIExportWrapper(index int, f *Function) {
	c.line("METHOD %s.", sanitizeABAP(f.ExportName))
	c.indent++
	c.emitWASIReset()
	params := make([]string, len(f.Type.Params))
	for i := range params {
		params[i] = fmt.Sprintf("p%d = p%d", i, i)
	}
	prefix := ""
	if len(f.Type.Results) > 0 {
		prefix = "rv = "
	}
	c.line("%sf%d( %s ).", prefix, index, strings.Join(params, " "))
	c.indent--
	c.line("ENDMETHOD.")
}
