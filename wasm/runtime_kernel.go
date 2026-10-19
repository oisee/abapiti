package wasm

import (
	"fmt"
	"regexp"
	"strings"
)

func kernelRuntimeBody(name string) string {
	switch name {
	case "div_s32", "rem_s32", "div_s64", "rem_s64":
		return signedDivisionBody(strings.HasSuffix(name, "64"), strings.HasPrefix(name, "rem"))
	case "div_u32", "rem_u32":
		op := "DIV"
		if name == "rem_u32" {
			op = "MOD"
		}
		return `DATA lv_a TYPE int8.
DATA lv_b TYPE int8.
DATA lv_result TYPE int8.
IF iv_b = 0. ` + wasmTrap + ` ENDIF.
lv_a = iv_a.
lv_b = iv_b.
IF lv_a < 0. lv_a = lv_a + 4294967296. ENDIF.
IF lv_b < 0. lv_b = lv_b + 4294967296. ENDIF.
lv_result = lv_a ` + op + ` lv_b.
IF lv_result >= 2147483648. lv_result = lv_result - 4294967296. ENDIF.
rv = lv_result.`
	case "div_u64", "rem_u64":
		return unsigned64DivisionBody(name == "rem_u64")
	case "lt_u64", "gt_u64", "le_u64", "ge_u64":
		op := map[string]string{"lt_u64": "<", "gt_u64": ">", "le_u64": "<=", "ge_u64": ">="}[name]
		sign := "<"
		if strings.HasPrefix(name, "gt") || strings.HasPrefix(name, "ge") {
			sign = ">"
		}
		return "IF iv_a < 0 AND iv_b >= 0.\n" +
			"rv = " + map[bool]string{true: "abap_true", false: "abap_false"}[sign == ">"] + ".\n" +
			"ELSEIF iv_a >= 0 AND iv_b < 0.\n" +
			"rv = " + map[bool]string{true: "abap_true", false: "abap_false"}[sign == "<"] + ".\n" +
			"ELSE.\nIF iv_a " + op + " iv_b. rv = abap_true. ELSE. rv = abap_false. ENDIF.\nENDIF."
	case "shl32":
		return `DATA lv_val TYPE int8.
DATA lv_shift TYPE i.
lv_val = iv_val.
lv_shift = iv_shift MOD 32.
DO lv_shift TIMES.
lv_val = lv_val MOD 2147483648 * 2.
IF lv_val >= 2147483648. lv_val = lv_val - 4294967296. ENDIF.
ENDDO.
rv = lv_val.`
	case "shr_u32":
		return `DATA lv_val TYPE int8.
DATA lv_shift TYPE i.
lv_val = iv_val.
IF lv_val < 0. lv_val = lv_val + 4294967296. ENDIF.
lv_shift = iv_shift MOD 32.
DO lv_shift TIMES.
lv_val = lv_val DIV 2.
ENDDO.
IF lv_val >= 2147483648. lv_val = lv_val - 4294967296. ENDIF.
rv = lv_val.`
	case "shr_s32", "shr_s64":
		return shiftRightSignedBody(name == "shr_s64")
	case "wrap_i64":
		return `DATA lv_val TYPE int8.
lv_val = iv_val MOD 4294967296.
IF lv_val >= 2147483648. lv_val = lv_val - 4294967296. ENDIF.
rv = lv_val.`
	case "clz64", "ctz64", "popcnt64":
		return count64Body(name)
	case "mem_ld_i64", "mem_ld_i32_16s":
		return integerLoadBody(name == "mem_ld_i64")
	case "mem_ld_f32", "mem_ld_f64", "mem_st_f32", "mem_st_f64":
		return floatMemoryBody(name)
	case "reinterpret_f32_i32", "reinterpret_f64_i64":
		return floatEncodeBody(name == "reinterpret_f64_i64")
	case "reinterpret_i32_f32", "reinterpret_i64_f64":
		return floatDecodeBody(name == "reinterpret_i64_f64")
	case "trunc_f_u32":
		return `DATA lv_val TYPE int8.
lv_val = trunc( iv_val ).
IF lv_val >= 2147483648. lv_val = lv_val - 4294967296. ENDIF.
rv = lv_val.`
	case "trunc_f_u64":
		return `DATA lv_val TYPE f.
lv_val = trunc( iv_val ).
IF lv_val >= '9223372036854775808'.
lv_val = lv_val - '9223372036854775808'.
rv = lv_val.
rv = rv - 9223372036854775807 - 1.
ELSE.
rv = lv_val.
ENDIF.`
	case "shl64", "shr_u64":
		return shift64Body(name == "shl64")
	case "rotl32", "rotr32", "rotl64", "rotr64":
		return rotateBody(name)
	case "mem_st_i64":
		return `DATA lv_be TYPE x LENGTH 8.
DATA lv_le TYPE x LENGTH 8.
lv_be = iv_val.
lv_le+0(1) = lv_be+7(1).
lv_le+1(1) = lv_be+6(1).
lv_le+2(1) = lv_be+5(1).
lv_le+3(1) = lv_be+4(1).
lv_le+4(1) = lv_be+3(1).
lv_le+5(1) = lv_be+2(1).
lv_le+6(1) = lv_be+1(1).
lv_le+7(1) = lv_be+0(1).
REPLACE SECTION OFFSET iv_addr LENGTH 8 OF cv_mem WITH lv_le IN BYTE MODE.`
	case "mem_ld_i64_ext":
		return `DATA lv_b1 TYPE x LENGTH 1.
DATA lv_b2 TYPE x LENGTH 2.
DATA lv_b4 TYPE x LENGTH 4.
DATA lv_be2 TYPE x LENGTH 2.
DATA lv_be4 TYPE x LENGTH 4.
CASE iv_op.
WHEN 48 OR 49.
  lv_b1 = iv_mem+iv_addr(1).
  rv = lv_b1.
  IF iv_op = 48 AND rv > 127. rv = rv - 256. ENDIF.
WHEN 50 OR 51.
  lv_b2 = iv_mem+iv_addr(2).
  lv_be2+0(1) = lv_b2+1(1).
  lv_be2+1(1) = lv_b2+0(1).
  rv = lv_be2.
  IF iv_op = 50 AND rv > 32767. rv = rv - 65536. ENDIF.
WHEN 52 OR 53.
  lv_b4 = iv_mem+iv_addr(4).
  lv_be4+0(1) = lv_b4+3(1).
  lv_be4+1(1) = lv_b4+2(1).
  lv_be4+2(1) = lv_b4+1(1).
  lv_be4+3(1) = lv_b4+0(1).
  rv = lv_be4.
  IF iv_op = 52 AND rv > 2147483647. rv = rv - 4294967296. ENDIF.
ENDCASE.`
	case "mem_st_i64_trunc":
		return `DATA lv_be TYPE x LENGTH 8.
DATA lv_b1 TYPE x LENGTH 1.
DATA lv_b2 TYPE x LENGTH 2.
DATA lv_b4 TYPE x LENGTH 4.
lv_be = iv_val.
CASE iv_op.
WHEN 60.
  lv_b1 = lv_be+7(1).
  REPLACE SECTION OFFSET iv_addr LENGTH 1 OF cv_mem WITH lv_b1 IN BYTE MODE.
WHEN 61.
  lv_b2+0(1) = lv_be+7(1).
  lv_b2+1(1) = lv_be+6(1).
  REPLACE SECTION OFFSET iv_addr LENGTH 2 OF cv_mem WITH lv_b2 IN BYTE MODE.
WHEN 62.
  lv_b4+0(1) = lv_be+7(1).
  lv_b4+1(1) = lv_be+6(1).
  lv_b4+2(1) = lv_be+5(1).
  lv_b4+3(1) = lv_be+4(1).
  REPLACE SECTION OFFSET iv_addr LENGTH 4 OF cv_mem WITH lv_b4 IN BYTE MODE.
ENDCASE.`
	}
	panic("unknown runtime helper: " + name)
}

// Halving the unsigned dividend keeps every intermediate within signed int8.
func unsigned64DivisionBody(remainder bool) string {
	result := "lv_q"
	if remainder {
		result = "lv_r"
	}
	return `DATA lv_half TYPE int8.
DATA lv_q TYPE int8.
DATA lv_r TYPE int8.
DATA lv_bit TYPE int8.
DATA lv_carry TYPE int8.
IF iv_b = 0. ` + wasmTrap + ` ENDIF.
IF iv_b < 0.
  lv_q = 0.
  lv_r = iv_a.
  IF iv_a < 0 AND iv_a >= iv_b.
    lv_q = 1.
    lv_r = iv_a - iv_b.
  ENDIF.
ELSEIF iv_b = 1.
  lv_q = iv_a.
  lv_r = 0.
ELSEIF iv_a >= 0.
  lv_q = iv_a DIV iv_b.
  lv_r = iv_a MOD iv_b.
ELSE.
  lv_half = iv_a + 9223372036854775807 + 1.
  lv_bit = lv_half MOD 2.
  lv_half = lv_half DIV 2 + 4611686018427387904.
  lv_q = lv_half DIV iv_b.
  lv_r = lv_half MOD iv_b.
  lv_carry = 0.
  IF lv_r >= iv_b - lv_r.
    lv_r = lv_r - ( iv_b - lv_r ) + lv_bit.
    lv_carry = 1.
  ELSE.
    lv_r = lv_r * 2 + lv_bit.
    IF lv_r >= iv_b.
      lv_r = lv_r - iv_b.
      lv_carry = 1.
    ENDIF.
  ENDIF.
  lv_q = lv_q * 2 + lv_carry.
ENDIF.
rv = ` + result + `.`
}

// Divide non-negative magnitudes, then restore the WASM sign. For INT64_MIN,
// divide |a|-1 and carry the missing unit through the remainder. Divisors of
// magnitude 2^63 and quotients of magnitude 2^63 are handled before negation.
func signedDivisionBody(is64, remainder bool) string {
	min := "-2147483648"
	if is64 {
		min = "( 0 - 9223372036854775807 - 1 )"
	}
	body := `DATA lv_a TYPE int8.
DATA lv_b TYPE int8.
DATA lv_q TYPE int8.
DATA lv_r TYPE int8.
IF iv_b = 0. ` + wasmTrap + ` ENDIF.
`
	if !remainder {
		body += "IF iv_a = " + min + " AND iv_b = -1. " + wasmTrap + " ENDIF.\n"
	}
	if is64 {
		body += "IF iv_b = " + min + ".\n"
		if remainder {
			body += "IF iv_a = iv_b. rv = 0. ELSE. rv = iv_a. ENDIF.\n"
		} else {
			body += "IF iv_a = iv_b. rv = 1. ELSE. rv = 0. ENDIF.\n"
		}
		body += "RETURN.\nENDIF.\n"
		body += "IF iv_a = " + min + " AND ( iv_b = 1 OR iv_b = -1 ).\n"
		if remainder {
			body += "rv = 0.\n"
		} else {
			body += "rv = iv_a.\n"
		}
		body += "RETURN.\nENDIF.\n"
	}
	body += `lv_b = iv_b.
IF lv_b < 0. lv_b = 0 - lv_b. ENDIF.
lv_a = iv_a.
`
	if is64 {
		body += "IF iv_a = " + min + ".\nlv_a = 0 - ( iv_a + 1 ).\nENDIF.\n"
	}
	body += `IF lv_a < 0. lv_a = 0 - lv_a. ENDIF.
lv_q = lv_a DIV lv_b.
lv_r = lv_a MOD lv_b.
`
	if is64 {
		body += "IF iv_a = " + min + ".\nlv_r = lv_r + 1.\nIF lv_r = lv_b.\nlv_q = lv_q + 1.\nlv_r = 0.\nENDIF.\nENDIF.\n"
	}
	if remainder {
		body += "IF iv_a < 0. lv_r = 0 - lv_r. ENDIF.\nrv = lv_r."
	} else {
		body += "IF ( iv_a < 0 AND iv_b > 0 ) OR ( iv_a >= 0 AND iv_b < 0 ).\nlv_q = 0 - lv_q.\nENDIF.\nrv = lv_q."
	}
	return body
}

func count64Body(name string) string {
	common := "DATA lv_val TYPE int8.\nlv_val = iv_val.\nrv = 0.\n"
	switch name {
	case "clz64":
		return common + `DATA lv_mask TYPE int8.
IF lv_val = 0. rv = 64. RETURN. ENDIF.
IF lv_val < 0. RETURN. ENDIF.
rv = 1.
lv_mask = 4611686018427387904.
WHILE lv_val < lv_mask.
rv = rv + 1.
lv_mask = lv_mask DIV 2.
ENDWHILE.`
	case "ctz64":
		return common + `IF lv_val = 0. rv = 64. RETURN. ENDIF.
WHILE lv_val MOD 2 = 0.
rv = rv + 1.
lv_val = lv_val DIV 2.
ENDWHILE.`
	default:
		return common + `DO 64 TIMES.
rv = rv + lv_val MOD 2.
lv_val = lv_val DIV 2.
ENDDO.`
	}
}

func integerLoadBody(is64 bool) string {
	n := 2
	if is64 {
		n = 8
	}
	lines := []string{fmt.Sprintf("DATA lv_le TYPE x LENGTH %d.", n), fmt.Sprintf("DATA lv_be TYPE x LENGTH %d.", n), fmt.Sprintf("lv_le = iv_mem+iv_addr(%d).", n)}
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("lv_be+%d(1) = lv_le+%d(1).", i, n-i-1))
	}
	lines = append(lines, "rv = lv_be.")
	if !is64 {
		lines = append(lines, "IF rv > 32767. rv = rv - 65536. ENDIF.")
	}
	return strings.Join(lines, "\n")
}

// IEEE fields are assembled arithmetically in int8; only fixed x fields are
// written by offset. The f32 significand uses round-to-nearest, ties-to-even.
func floatEncodeBody(is64 bool) string {
	bias, minExp, fraction := "127", "-126", "8388608"
	if is64 {
		bias, minExp, fraction = "1023", "-1022", "4503599627370496"
	}
	body := `DATA lv_mag TYPE f.
DATA lv_scaled TYPE f.
DATA lv_fraction TYPE f.
DATA lv_exp TYPE i.
DATA lv_sign TYPE int8.
DATA lv_frac TYPE int8.
DATA lv_hi TYPE int8.
DATA lv_lo TYPE int8.
lv_mag = abs( iv_val ).
lv_sign = 0.
IF iv_val < 0. lv_sign = 1. ENDIF.
lv_exp = 0.
IF lv_mag = 0.
rv = 0.
ELSE.
WHILE lv_mag >= 2.
lv_mag = lv_mag / 2.
lv_exp = lv_exp + 1.
ENDWHILE.
WHILE lv_mag < 1 AND lv_exp > ` + minExp + `.
lv_mag = lv_mag * 2.
lv_exp = lv_exp - 1.
ENDWHILE.
IF lv_mag < 1.
lv_exp = 0.
lv_scaled = lv_mag * '` + fraction + `'.
ELSE.
lv_exp = lv_exp + ` + bias + `.
lv_scaled = ( lv_mag - 1 ) * '` + fraction + `'.
ENDIF.
lv_frac = trunc( lv_scaled ).
lv_fraction = lv_frac.
lv_fraction = lv_scaled - lv_fraction.
IF lv_fraction > '0.5'.
lv_frac = lv_frac + 1.
ELSEIF lv_fraction = '0.5' AND lv_frac MOD 2 = 1.
lv_frac = lv_frac + 1.
ENDIF.
IF lv_frac >= ` + fraction + `.
lv_frac = 0.
lv_exp = lv_exp + 1.
ENDIF.
`
	if is64 {
		return body + `lv_hi = lv_exp * 1048576 + lv_frac DIV 4294967296.
IF lv_sign = 1. lv_hi = lv_hi - 2147483648. ENDIF.
lv_lo = lv_frac MOD 4294967296.
rv = lv_hi * 4294967296 + lv_lo.
ENDIF.`
	}
	return body + `lv_hi = lv_exp * 8388608 + lv_frac.
IF lv_sign = 1. lv_hi = lv_hi - 2147483648. ENDIF.
rv = lv_hi.
ENDIF.`
}

func floatDecodeBody(is64 bool) string {
	body := `DATA lv_bits TYPE int8.
DATA lv_exp TYPE i.
DATA lv_frac TYPE int8.
DATA lv_sign TYPE i.
DATA lv_power TYPE i.
lv_bits = iv_val.
lv_sign = 0.
IF lv_bits < 0. lv_sign = 1. ENDIF.
`
	if is64 {
		body += `lv_exp = lv_bits DIV 4503599627370496 MOD 2048.
IF lv_exp = 2047. ` + wasmTrap + ` ENDIF.
lv_frac = lv_bits MOD 4503599627370496.
rv = lv_frac.
rv = rv / '4503599627370496'.
IF lv_exp = 0. lv_power = -1022. ELSE. lv_power = lv_exp - 1023. rv = rv + 1. ENDIF.
`
	} else {
		body += `lv_exp = lv_bits DIV 8388608 MOD 256.
IF lv_exp = 255. ` + wasmTrap + ` ENDIF.
lv_frac = lv_bits MOD 8388608.
rv = lv_frac.
rv = rv / 8388608.
IF lv_exp = 0. lv_power = -126. ELSE. lv_power = lv_exp - 127. rv = rv + 1. ENDIF.
`
	}
	return body + `WHILE lv_power > 0.
rv = rv * 2.
lv_power = lv_power - 1.
ENDWHILE.
WHILE lv_power < 0.
rv = rv / 2.
lv_power = lv_power + 1.
ENDWHILE.
IF lv_sign = 1. rv = - rv. ENDIF.`
}

func floatMemoryBody(name string) string {
	is64 := strings.HasSuffix(name, "64")
	n, typ := 4, "i"
	if is64 {
		n, typ = 8, "int8"
	}
	lines := []string{fmt.Sprintf("DATA lv_be TYPE x LENGTH %d.", n), fmt.Sprintf("DATA lv_le TYPE x LENGTH %d.", n)}
	if strings.HasPrefix(name, "mem_ld") {
		lines = append(lines, "DATA lv_value TYPE "+typ+".", fmt.Sprintf("lv_le = iv_mem+iv_addr(%d).", n))
		for i := 0; i < n; i++ {
			lines = append(lines, fmt.Sprintf("lv_be+%d(1) = lv_le+%d(1).", i, n-1-i))
		}
		lines = append(lines, "lv_value = lv_be.", strings.ReplaceAll(floatDecodeBody(is64), "iv_val", "lv_value"))
	} else {
		lines = append(lines, "DATA lv_value TYPE "+typ+".", regexp.MustCompile(`\brv\b`).ReplaceAllString(floatEncodeBody(is64), "lv_value"), "lv_be = lv_value.")
		for i := 0; i < n; i++ {
			lines = append(lines, fmt.Sprintf("lv_le+%d(1) = lv_be+%d(1).", i, n-1-i))
		}
		lines = append(lines, fmt.Sprintf("REPLACE SECTION OFFSET iv_addr LENGTH %d OF cv_mem WITH lv_le IN BYTE MODE.", n))
	}
	return strings.Join(lines, "\n")
}
