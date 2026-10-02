package wasm

import "strings"

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
DATA mv_random TYPE int8.
DATA mt_args TYPE string_table.
DATA mt_env TYPE string_table.
METHODS get_stdout RETURNING VALUE(rv) TYPE xstring.
METHODS get_stderr RETURNING VALUE(rv) TYPE xstring.
METHODS set_stdin IMPORTING iv TYPE xstring.
METHODS get_exit_code RETURNING VALUE(rv) TYPE i.
METHODS set_args IMPORTING it TYPE string_table.
METHODS set_env IMPORTING it TYPE string_table.
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
METHOD wasi_call.
DATA lv_addr TYPE i.
DATA lv_ptr TYPE i.
DATA lv_len TYPE i.
DATA lv_total TYPE i.
DATA lv_count TYPE i.
DATA lv_buf TYPE i.
DATA lv_left TYPE i.
DATA lv_bytes TYPE xstring.
DATA lv_zero TYPE x LENGTH 1.
DATA lv_stat TYPE x LENGTH 24.
DATA lv_strings TYPE string_table.
DATA lv_string TYPE string.
DATA lo_utf8 TYPE REF TO cl_abap_conv_out_ce.
DATA lv_ts TYPE timestampl.
DATA lv_whole TYPE p LENGTH 8 DECIMALS 0.
DATA lv_text TYPE n LENGTH 14.
DATA lv_date TYPE d.
DATA lv_epoch TYPE d VALUE '19700101'.
DATA lv_days TYPE i.
DATA lv_hours TYPE i.
DATA lv_minutes TYPE i.
DATA lv_seconds TYPE i.
DATA lv_ns TYPE int8.
DATA lv_fraction TYPE p LENGTH 8 DECIMALS 7.
DATA lv_runtime TYPE i.
rv = 0.
CASE iv_name.
WHEN 'fd_write' OR 'fd_read'.
IF iv_name = 'fd_write'.
IF p0 <> 1 AND p0 <> 2. rv = 8. RETURN. ENDIF.
ELSE.
IF p0 <> 0. rv = 8. RETURN. ENDIF.
ENDIF.
lv_count = p2.
lv_addr = p1.
DO lv_count TIMES.
lv_ptr = mem_ld_i32( lv_addr ).
lv_len = mem_ld_i32( lv_addr + 4 ).
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
mem_st_i32( iv_addr = p3 iv_val = lv_total ).
WHEN 'fd_close' OR 'sched_yield'.
rv = 0.
WHEN 'fd_seek'.
IF p0 >= 0 AND p0 <= 2. rv = 70. ELSE. rv = 8. ENDIF.
WHEN 'fd_fdstat_get'.
IF p0 < 0 OR p0 > 2. rv = 8. RETURN. ENDIF.
lv_addr = p1.
REPLACE SECTION OFFSET lv_addr LENGTH 24 OF mv_mem WITH lv_stat IN BYTE MODE.
mem_st_i32_8( iv_addr = lv_addr iv_val = 2 ).
mem_st_i32_16( iv_addr = lv_addr + 2 iv_val = 0 ).
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = lv_addr + 8 iv_val = -1 CHANGING cv_mem = mv_mem ).
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = lv_addr + 16 iv_val = -1 CHANGING cv_mem = mv_mem ).
WHEN 'fd_prestat_get' OR 'fd_prestat_dir_name'.
rv = 8.
WHEN 'args_sizes_get' OR 'environ_sizes_get' OR 'args_get' OR 'environ_get'.
IF iv_name = 'args_sizes_get' OR iv_name = 'args_get'.
lv_strings = mt_args.
ELSE.
lv_strings = mt_env.
ENDIF.
lv_addr = p0.
lv_buf = p1.
LOOP AT lv_strings INTO lv_string.
lo_utf8 = cl_abap_conv_out_ce=>create( encoding = 'UTF-8' ).
lo_utf8->write( data = lv_string ).
lv_bytes = lo_utf8->get_buffer( ).
CONCATENATE lv_bytes lv_zero INTO lv_bytes IN BYTE MODE.
lv_len = xstrlen( lv_bytes ).
IF iv_name = 'args_get' OR iv_name = 'environ_get'.
mem_st_i32( iv_addr = lv_addr iv_val = lv_buf ).
REPLACE SECTION OFFSET lv_buf LENGTH lv_len OF mv_mem WITH lv_bytes IN BYTE MODE.
lv_addr = lv_addr + 4.
lv_buf = lv_buf + lv_len.
ENDIF.
lv_total = lv_total + lv_len.
ENDLOOP.
IF iv_name = 'args_sizes_get' OR iv_name = 'environ_sizes_get'.
DESCRIBE TABLE lv_strings LINES lv_count.
mem_st_i32( iv_addr = p0 iv_val = lv_count ).
lv_addr = p1.
mem_st_i32( iv_addr = lv_addr iv_val = lv_total ).
ENDIF.
WHEN 'clock_time_get'.
IF p0 = 0.
GET TIME STAMP FIELD lv_ts.
lv_whole = trunc( lv_ts ).
lv_fraction = lv_ts - lv_whole.
lv_text = lv_whole.
lv_date = lv_text+0(8).
lv_days = lv_date - lv_epoch.
lv_hours = lv_text+8(2).
lv_minutes = lv_text+10(2).
lv_seconds = lv_text+12(2).
lv_ns = lv_days.
lv_ns = ( lv_ns * 86400 + lv_hours * 3600 + lv_minutes * 60 + lv_seconds ) * 1000000000.
lv_ns = lv_ns + lv_fraction * 1000000000.
ELSEIF p0 = 1.
GET RUN TIME FIELD lv_runtime.
lv_ns = lv_runtime.
IF lv_ns < 0. lv_ns = lv_ns + 4294967296. ENDIF.
lv_ns = lv_ns * 1000.
ELSE.
rv = 28. RETURN.
ENDIF.
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = p2 iv_val = lv_ns CHANGING cv_mem = mv_mem ).
WHEN 'clock_res_get'.
IF p0 <> 0 AND p0 <> 1. rv = 28. RETURN. ENDIF.
lv_addr = p1.
zcl_wasm_rt=>mem_st_i64( EXPORTING iv_addr = lv_addr iv_val = 1000 CHANGING cv_mem = mv_mem ).
WHEN 'random_get'.
lv_addr = p0.
lv_count = p1.
DO lv_count TIMES.
mv_random = ( mv_random * 1664525 + 1013904223 ) MOD 4294967296.
lv_len = mv_random DIV 16777216.
mem_st_i32_8( iv_addr = lv_addr iv_val = lv_len ).
lv_addr = lv_addr + 1.
ENDDO.
WHEN 'proc_exit'.
mv_exit_code = p0.
RAISE EXCEPTION TYPE cx_sy_dyn_call_illegal_method.
WHEN OTHERS.
rv = 52.
ENDCASE.
ENDMETHOD.`
	for _, line := range strings.Split(body, "\n") {
		c.line("%s", line)
	}
}
