package wasm

import "strings"

func emitRuntimeClass() string {
	src := `CLASS zcl_wasm_rt DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    CLASS-METHODS i64_add IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS i64_sub IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS i64_mul IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    " Memory allocation
    CLASS-METHODS alloc_mem IMPORTING iv_size TYPE i RETURNING VALUE(rv_mem) TYPE xstring.
    CLASS-METHODS mem_init IMPORTING iv_off TYPE i iv_hex TYPE string CHANGING cv_mem TYPE xstring.
    CLASS-METHODS mem_copy IMPORTING iv_dst TYPE i iv_src TYPE i iv_n TYPE i CHANGING cv_mem TYPE xstring.
    CLASS-METHODS mem_fill IMPORTING iv_dst TYPE i iv_val TYPE i iv_n TYPE i CHANGING cv_mem TYPE xstring.
    " Unsigned 32-bit ops
    CLASS-METHODS div_u32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS rem_u32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS lt_u32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE abap_bool.
    CLASS-METHODS gt_u32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE abap_bool.
    CLASS-METHODS le_u32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE abap_bool.
    CLASS-METHODS ge_u32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE abap_bool.
    " Bitwise 32
    CLASS-METHODS and32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS or32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS xor32 IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS shl32 IMPORTING iv_val TYPE i iv_shift TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS shr_s32 IMPORTING iv_val TYPE i iv_shift TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS shr_u32 IMPORTING iv_val TYPE i iv_shift TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS rotl32 IMPORTING iv_val TYPE i iv_shift TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS rotr32 IMPORTING iv_val TYPE i iv_shift TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS clz32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS ctz32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS popcnt32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE i.
    " Unsigned 64-bit ops
    CLASS-METHODS div_u64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS rem_u64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS lt_u64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE abap_bool.
    CLASS-METHODS gt_u64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE abap_bool.
    CLASS-METHODS le_u64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE abap_bool.
    CLASS-METHODS ge_u64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE abap_bool.
    " Bitwise 64
    CLASS-METHODS and64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS or64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS xor64 IMPORTING iv_a TYPE int8 iv_b TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS shl64 IMPORTING iv_val TYPE int8 iv_shift TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS shr_s64 IMPORTING iv_val TYPE int8 iv_shift TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS shr_u64 IMPORTING iv_val TYPE int8 iv_shift TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS rotl64 IMPORTING iv_val TYPE int8 iv_shift TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS rotr64 IMPORTING iv_val TYPE int8 iv_shift TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS clz64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS ctz64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS popcnt64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE int8.
    " Conversions
    CLASS-METHODS wrap_i64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS extend_u32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS extend_u64_f IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE f.
    CLASS-METHODS trunc_f_u32 IMPORTING iv_val TYPE f RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS trunc_f_u64 IMPORTING iv_val TYPE f RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS extend8s_i32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS extend16s_i32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS extend8s_i64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS extend16s_i64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS extend32s_i64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS copysign IMPORTING iv_mag TYPE f iv_sign TYPE f RETURNING VALUE(rv) TYPE f.
    CLASS-METHODS reinterpret_f32_i32 IMPORTING iv_val TYPE f RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS reinterpret_i32_f32 IMPORTING iv_val TYPE i RETURNING VALUE(rv) TYPE f.
    CLASS-METHODS reinterpret_f64_i64 IMPORTING iv_val TYPE f RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS reinterpret_i64_f64 IMPORTING iv_val TYPE int8 RETURNING VALUE(rv) TYPE f.
    " Memory load/store for i64, f32, f64
    CLASS-METHODS mem_ld_i64 IMPORTING iv_mem TYPE xstring iv_addr TYPE i RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS mem_st_i64 IMPORTING iv_val TYPE int8 iv_addr TYPE i CHANGING cv_mem TYPE xstring.
    CLASS-METHODS mem_ld_i64_ext IMPORTING iv_mem TYPE xstring iv_addr TYPE i iv_op TYPE i RETURNING VALUE(rv) TYPE int8.
    CLASS-METHODS mem_st_i64_trunc IMPORTING iv_val TYPE int8 iv_addr TYPE i iv_op TYPE i CHANGING cv_mem TYPE xstring.
    CLASS-METHODS mem_ld_i32_16s IMPORTING iv_mem TYPE xstring iv_addr TYPE i RETURNING VALUE(rv) TYPE i.
    CLASS-METHODS mem_ld_f32 IMPORTING iv_mem TYPE xstring iv_addr TYPE i RETURNING VALUE(rv) TYPE f.
    CLASS-METHODS mem_ld_f64 IMPORTING iv_mem TYPE xstring iv_addr TYPE i RETURNING VALUE(rv) TYPE f.
    CLASS-METHODS mem_st_f32 IMPORTING iv_val TYPE f iv_addr TYPE i CHANGING cv_mem TYPE xstring.
    CLASS-METHODS mem_st_f64 IMPORTING iv_val TYPE f iv_addr TYPE i CHANGING cv_mem TYPE xstring.
ENDCLASS.

CLASS zcl_wasm_rt IMPLEMENTATION.
` + emitI64RuntimeMethods() + `  METHOD alloc_mem.
    " Allocate iv_size bytes of zeroed memory
    DATA lv_hex TYPE string.
    DATA lv_chunk TYPE x LENGTH 256.
    DATA(lv_chunks) = iv_size DIV 256.
    DATA(lv_remainder) = iv_size MOD 256.
    DO lv_chunks TIMES.
      CONCATENATE rv_mem lv_chunk INTO rv_mem IN BYTE MODE.
    ENDDO.
    IF lv_remainder > 0.
      DATA lv_small TYPE x LENGTH 1.
      DO lv_remainder TIMES.
        CONCATENATE rv_mem lv_small INTO rv_mem IN BYTE MODE.
      ENDDO.
    ENDIF.
  ENDMETHOD.

  METHOD mem_init.
    DATA lv_data TYPE xstring.
    DATA lv_bytes TYPE i.
    lv_data = iv_hex.
    lv_bytes = xstrlen( lv_data ).
    REPLACE SECTION OFFSET iv_off LENGTH lv_bytes OF cv_mem WITH lv_data IN BYTE MODE.
  ENDMETHOD.

  METHOD mem_copy.
    IF iv_n <= 0. RETURN. ENDIF.
    DATA(lv_src_data) = cv_mem+iv_src(iv_n).
    REPLACE SECTION OFFSET iv_dst LENGTH iv_n OF cv_mem WITH lv_src_data IN BYTE MODE.
  ENDMETHOD.

  METHOD mem_fill.
    IF iv_n <= 0. RETURN. ENDIF.
    DATA lv_byte TYPE x LENGTH 1.
    lv_byte = iv_val.
    DATA lv_fill TYPE xstring.
    DATA lv_len TYPE i.
    DATA lv_remaining TYPE i.
    lv_fill = lv_byte.
    lv_len = 1.
    WHILE lv_len < iv_n.
      lv_remaining = iv_n - lv_len.
      IF lv_remaining >= lv_len.
        CONCATENATE lv_fill lv_fill INTO lv_fill IN BYTE MODE.
        lv_len = lv_len + lv_len.
      ELSE.
        CONCATENATE lv_fill lv_fill+0(lv_remaining) INTO lv_fill IN BYTE MODE.
        lv_len = iv_n.
      ENDIF.
    ENDWHILE.
    REPLACE SECTION OFFSET iv_dst LENGTH iv_n OF cv_mem WITH lv_fill IN BYTE MODE.
  ENDMETHOD.

  " === Unsigned 32-bit via INT8 promotion ===
  METHOD div_u32.
` + kernelRuntimeBody("div_u32") + `
  ENDMETHOD.
  METHOD rem_u32.
` + kernelRuntimeBody("rem_u32") + `
  ENDMETHOD.
  METHOD lt_u32.
    DATA(lv_a) = CONV int8( iv_a ).
    DATA(lv_b) = CONV int8( iv_b ).
    IF lv_a < 0. lv_a = lv_a + 4294967296. ENDIF.
    IF lv_b < 0. lv_b = lv_b + 4294967296. ENDIF.
    rv = xsdbool( lv_a < lv_b ).
  ENDMETHOD.
  METHOD gt_u32.
    DATA(lv_a) = CONV int8( iv_a ).
    DATA(lv_b) = CONV int8( iv_b ).
    IF lv_a < 0. lv_a = lv_a + 4294967296. ENDIF.
    IF lv_b < 0. lv_b = lv_b + 4294967296. ENDIF.
    rv = xsdbool( lv_a > lv_b ).
  ENDMETHOD.
  METHOD le_u32.
    DATA(lv_a) = CONV int8( iv_a ).
    DATA(lv_b) = CONV int8( iv_b ).
    IF lv_a < 0. lv_a = lv_a + 4294967296. ENDIF.
    IF lv_b < 0. lv_b = lv_b + 4294967296. ENDIF.
    rv = xsdbool( lv_a <= lv_b ).
  ENDMETHOD.
  METHOD ge_u32.
    DATA(lv_a) = CONV int8( iv_a ).
    DATA(lv_b) = CONV int8( iv_b ).
    IF lv_a < 0. lv_a = lv_a + 4294967296. ENDIF.
    IF lv_b < 0. lv_b = lv_b + 4294967296. ENDIF.
    rv = xsdbool( lv_a >= lv_b ).
  ENDMETHOD.

  " === Bitwise 32 (via XSTRING) ===
  METHOD and32.
    DATA lv_a TYPE x LENGTH 4. DATA lv_b TYPE x LENGTH 4. DATA lv_r TYPE x LENGTH 4.
    lv_a = iv_a. lv_b = iv_b.
    lv_r = lv_a BIT-AND lv_b.
    rv = lv_r.
  ENDMETHOD.
  METHOD or32.
    DATA lv_a TYPE x LENGTH 4. DATA lv_b TYPE x LENGTH 4. DATA lv_r TYPE x LENGTH 4.
    lv_a = iv_a. lv_b = iv_b.
    lv_r = lv_a BIT-OR lv_b.
    rv = lv_r.
  ENDMETHOD.
  METHOD xor32.
    DATA lv_a TYPE x LENGTH 4. DATA lv_b TYPE x LENGTH 4. DATA lv_r TYPE x LENGTH 4.
    lv_a = iv_a. lv_b = iv_b.
    lv_r = lv_a BIT-XOR lv_b.
    rv = lv_r.
  ENDMETHOD.
  METHOD shl32.
` + kernelRuntimeBody("shl32") + `
  ENDMETHOD.
  METHOD shr_s32.
` + kernelRuntimeBody("shr_s32") + `
  ENDMETHOD.
  METHOD shr_u32.
` + kernelRuntimeBody("shr_u32") + `
  ENDMETHOD.
  METHOD rotl32.
` + kernelRuntimeBody("rotl32") + `
  ENDMETHOD.
  METHOD rotr32.
` + kernelRuntimeBody("rotr32") + `
  ENDMETHOD.
  METHOD clz32.
    DATA(lv_val) = CONV int8( iv_val ).
    IF lv_val < 0. lv_val = lv_val + 4294967296. ENDIF.
    IF lv_val = 0. rv = 32. RETURN. ENDIF.
    rv = 0.
    DATA lv_mask TYPE int8.
    lv_mask = 2147483648.
    WHILE lv_val < lv_mask.
      rv = rv + 1.
      lv_mask = lv_mask DIV 2.
    ENDWHILE.
  ENDMETHOD.
  METHOD ctz32.
    DATA(lv_val) = CONV int8( iv_val ).
    IF lv_val < 0. lv_val = lv_val + 4294967296. ENDIF.
    IF lv_val = 0. rv = 32. RETURN. ENDIF.
    rv = 0.
    WHILE lv_val MOD 2 = 0.
      rv = rv + 1.
      lv_val = lv_val DIV 2.
    ENDWHILE.
  ENDMETHOD.
  METHOD popcnt32.
    DATA(lv_val) = CONV int8( iv_val ).
    IF lv_val < 0. lv_val = lv_val + 4294967296. ENDIF.
    rv = 0.
    WHILE lv_val > 0.
      IF lv_val MOD 2 = 1. rv = rv + 1. ENDIF.
      lv_val = lv_val DIV 2.
    ENDWHILE.
  ENDMETHOD.

  " === 64-bit stubs (implement as needed) ===
  METHOD div_u64.
` + kernelRuntimeBody("div_u64") + `
  ENDMETHOD.
  METHOD rem_u64.
` + kernelRuntimeBody("rem_u64") + `
  ENDMETHOD.
  METHOD lt_u64.
` + kernelRuntimeBody("lt_u64") + `
  ENDMETHOD.
  METHOD gt_u64.
` + kernelRuntimeBody("gt_u64") + `
  ENDMETHOD.
  METHOD le_u64.
` + kernelRuntimeBody("le_u64") + `
  ENDMETHOD.
  METHOD ge_u64.
` + kernelRuntimeBody("ge_u64") + `
  ENDMETHOD.
  METHOD and64. DATA lv_a TYPE x LENGTH 8. DATA lv_b TYPE x LENGTH 8. lv_a = iv_a. lv_b = iv_b. DATA(lv_r) = lv_a BIT-AND lv_b. rv = lv_r. ENDMETHOD.
  METHOD or64. DATA lv_a TYPE x LENGTH 8. DATA lv_b TYPE x LENGTH 8. lv_a = iv_a. lv_b = iv_b. DATA(lv_r) = lv_a BIT-OR lv_b. rv = lv_r. ENDMETHOD.
  METHOD xor64. DATA lv_a TYPE x LENGTH 8. DATA lv_b TYPE x LENGTH 8. lv_a = iv_a. lv_b = iv_b. DATA(lv_r) = lv_a BIT-XOR lv_b. rv = lv_r. ENDMETHOD.
  METHOD shl64.
` + kernelRuntimeBody("shl64") + `
  ENDMETHOD.
  METHOD shr_s64.
` + kernelRuntimeBody("shr_s64") + `
  ENDMETHOD.
  METHOD shr_u64.
` + kernelRuntimeBody("shr_u64") + `
  ENDMETHOD.
  METHOD rotl64.
` + kernelRuntimeBody("rotl64") + `
  ENDMETHOD.
  METHOD rotr64.
` + kernelRuntimeBody("rotr64") + `
  ENDMETHOD.
  METHOD clz64.
` + kernelRuntimeBody("clz64") + `
  ENDMETHOD.
  METHOD ctz64.
` + kernelRuntimeBody("ctz64") + `
  ENDMETHOD.
  METHOD popcnt64.
` + kernelRuntimeBody("popcnt64") + `
  ENDMETHOD.

  " === Conversions ===
  METHOD wrap_i64.
` + kernelRuntimeBody("wrap_i64") + `
  ENDMETHOD.
  METHOD extend_u32. rv = iv_val. IF rv < 0. rv = rv + 4294967296. ENDIF. ENDMETHOD.
  METHOD extend_u64_f. rv = iv_val. IF rv < 0. rv = rv + CONV f( '18446744073709551616' ). ENDIF. ENDMETHOD.
  METHOD trunc_f_u32.
` + kernelRuntimeBody("trunc_f_u32") + `
  ENDMETHOD.
  METHOD trunc_f_u64.
` + kernelRuntimeBody("trunc_f_u64") + `
  ENDMETHOD.
  METHOD extend8s_i32.
    rv = iv_val MOD 256.
    IF rv > 127. rv = rv - 256. ENDIF.
  ENDMETHOD.
  METHOD extend16s_i32.
    rv = iv_val MOD 65536.
    IF rv > 32767. rv = rv - 65536. ENDIF.
  ENDMETHOD.
  METHOD extend8s_i64. rv = iv_val MOD 256. IF rv > 127. rv = rv - 256. ENDIF. ENDMETHOD.
  METHOD extend16s_i64. rv = iv_val MOD 65536. IF rv > 32767. rv = rv - 65536. ENDIF. ENDMETHOD.
  METHOD extend32s_i64. rv = iv_val MOD 4294967296. IF rv > 2147483647. rv = rv - 4294967296. ENDIF. ENDMETHOD.
  METHOD copysign.
    rv = abs( iv_mag ).
    IF iv_sign < 0. rv = - rv. ENDIF.
  ENDMETHOD.
  METHOD reinterpret_f32_i32.
` + kernelRuntimeBody("reinterpret_f32_i32") + `
  ENDMETHOD.
  METHOD reinterpret_i32_f32.
` + kernelRuntimeBody("reinterpret_i32_f32") + `
  ENDMETHOD.
  METHOD reinterpret_f64_i64.
` + kernelRuntimeBody("reinterpret_f64_i64") + `
  ENDMETHOD.
  METHOD reinterpret_i64_f64.
` + kernelRuntimeBody("reinterpret_i64_f64") + `
  ENDMETHOD.

  " === Memory i64/f32/f64 ===
  METHOD mem_ld_i64.
` + kernelRuntimeBody("mem_ld_i64") + `
  ENDMETHOD.
  METHOD mem_st_i64.
` + kernelRuntimeBody("mem_st_i64") + `
  ENDMETHOD.
  METHOD mem_ld_i64_ext.
` + kernelRuntimeBody("mem_ld_i64_ext") + `
  ENDMETHOD.
  METHOD mem_st_i64_trunc.
` + kernelRuntimeBody("mem_st_i64_trunc") + `
  ENDMETHOD.
  METHOD mem_ld_i32_16s.
` + kernelRuntimeBody("mem_ld_i32_16s") + `
  ENDMETHOD.
  METHOD mem_ld_f32.
` + kernelRuntimeBody("mem_ld_f32") + `
  ENDMETHOD.
  METHOD mem_ld_f64.
` + kernelRuntimeBody("mem_ld_f64") + `
  ENDMETHOD.
  METHOD mem_st_f32.
` + kernelRuntimeBody("mem_st_f32") + `
  ENDMETHOD.
  METHOD mem_st_f64.
` + kernelRuntimeBody("mem_st_f64") + `
  ENDMETHOD.
ENDCLASS.
`
	return stripABAPComments(runtimeMethodRE.ReplaceAllStringFunc(src, func(method string) string {
		m := runtimeMethodRE.FindStringSubmatch(method)
		return "METHOD " + m[1] + ".\n" + legacyRuntimeBody(m[1], strings.TrimSpace(m[2])) + "\nENDMETHOD."
	}))
}
