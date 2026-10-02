package wasm

import (
	"fmt"
	"strings"
)

// Linear memory is an xstring. ABAP does not allow an offset/length on a
// string or xstring in a writer position (`mv_mem+iv_addr(4) = ...` is a
// syntax error on a real kernel and on open-steamgate: "xstring/string
// offset/length in writer position not possible"), so every write goes through
// REPLACE SECTION ... IN BYTE MODE. Reads with an offset are allowed.
//
// Byte order: an i32 converted to x LENGTH 4 is big-endian two's complement,
// and x LENGTH 4 converted back to i is signed; WASM memory is little-endian,
// so the helpers swap bytes inside a fixed-length x field (where offset writes
// are allowed). x LENGTH 1/2 to i is unsigned (padded with 00 on the left).

// memHelper is one memory helper: its name, whether it returns a value (rv),
// and its body. The body uses {mem} and {pages} for the memory variables.
type memHelper struct {
	name    string
	returns bool
	body    []string
}

var memHelperDefs = []memHelper{
	{"mem_ld_i32", true, []string{
		"DATA lv_le TYPE x LENGTH 4.",
		"DATA lv_be TYPE x LENGTH 4.",
		"lv_le = {mem}+iv_addr(4).",
		"lv_be+0(1) = lv_le+3(1).",
		"lv_be+1(1) = lv_le+2(1).",
		"lv_be+2(1) = lv_le+1(1).",
		"lv_be+3(1) = lv_le+0(1).",
		"rv = lv_be.",
	}},
	{"mem_st_i32", false, []string{
		"DATA lv_le TYPE x LENGTH 4.",
		"DATA lv_be TYPE x LENGTH 4.",
		"lv_be = iv_val.",
		"lv_le+0(1) = lv_be+3(1).",
		"lv_le+1(1) = lv_be+2(1).",
		"lv_le+2(1) = lv_be+1(1).",
		"lv_le+3(1) = lv_be+0(1).",
		"REPLACE SECTION OFFSET iv_addr LENGTH 4 OF {mem} WITH lv_le IN BYTE MODE.",
	}},
	{"mem_ld_i32_8u", true, []string{
		"DATA lv_b TYPE x LENGTH 1.",
		"lv_b = {mem}+iv_addr(1).",
		"rv = lv_b.",
	}},
	{"mem_ld_i32_8s", true, []string{
		"DATA lv_b TYPE x LENGTH 1.",
		"lv_b = {mem}+iv_addr(1).",
		"rv = lv_b.",
		"IF rv > 127. rv = rv - 256. ENDIF.",
	}},
	{"mem_ld_i32_16u", true, []string{
		"DATA lv_le TYPE x LENGTH 2.",
		"DATA lv_be TYPE x LENGTH 2.",
		"lv_le = {mem}+iv_addr(2).",
		"lv_be+0(1) = lv_le+1(1).",
		"lv_be+1(1) = lv_le+0(1).",
		"rv = lv_be.",
	}},
	{"mem_st_i32_8", false, []string{
		"DATA lv_b TYPE x LENGTH 1.",
		"lv_b = iv_val.",
		"REPLACE SECTION OFFSET iv_addr LENGTH 1 OF {mem} WITH lv_b IN BYTE MODE.",
	}},
	{"mem_st_i32_16", false, []string{
		"DATA lv_le TYPE x LENGTH 2.",
		"DATA lv_be TYPE x LENGTH 2.",
		"lv_be = iv_val.",
		"lv_le+0(1) = lv_be+1(1).",
		"lv_le+1(1) = lv_be+0(1).",
		"REPLACE SECTION OFFSET iv_addr LENGTH 2 OF {mem} WITH lv_le IN BYTE MODE.",
	}},
	{"mem_grow", true, []string{
		"IF iv_pages < 0.",
		"  rv = -1.",
		"  RETURN.",
		"ENDIF.",
		"IF iv_pages > {limit} - {pages}.",
		"  rv = -1.",
		"  RETURN.",
		"ENDIF.",
		"rv = {pages}.",
		"IF iv_pages = 0. RETURN. ENDIF.",
		"{zero_call}",
		"CONCATENATE {mem} lv_zeros INTO {mem} IN BYTE MODE.",
		"{pages} = {pages} + iv_pages.",
	}},
}

// zeroPagesBody builds one zero page in eight doublings, then appends pages.
// The page loop avoids making an oversized temporary for large memories.
func zeroPagesBody() []string {
	return []string{
		"DATA lv_chunk TYPE x LENGTH 256.",
		"DATA lv_page TYPE xstring.",
		"IF iv_pages = 0. RETURN. ENDIF.",
		"lv_page = lv_chunk.",
		"DO 8 TIMES.",
		"  CONCATENATE lv_page lv_page INTO lv_page IN BYTE MODE.",
		"ENDDO.",
		"DO iv_pages TIMES.",
		"  CONCATENATE rv_mem lv_page INTO rv_mem IN BYTE MODE.",
		"ENDDO.",
	}
}

func memoryLimit(m *Memory) int {
	if m != nil && m.HasMax {
		return m.Max
	}
	return 65536
}

// memHelperBody returns a helper body with the memory variables filled in.
func memHelperBody(h memHelper, mem, pages string, limit int, fugr bool) []string {
	out := make([]string, len(h.body))
	zeroCall := "DATA lv_zeros TYPE xstring. lv_zeros = mem_zero_pages( iv_pages )."
	if fugr {
		zeroCall = "DATA lv_zeros TYPE xstring. PERFORM mem_zero_pages USING iv_pages CHANGING lv_zeros."
	}
	r := strings.NewReplacer("{mem}", mem, "{pages}", pages, "{limit}", fmt.Sprint(limit), "{zero_call}", zeroCall)
	for i, l := range h.body {
		out[i] = r.Replace(l)
	}
	return out
}

// memHelperParams is the FORM parameter list of a helper.
func memHelperParams(h memHelper) string {
	switch {
	case h.name == "mem_grow":
		return "USING iv_pages TYPE i CHANGING rv TYPE i"
	case h.returns:
		return "USING iv_addr TYPE i CHANGING rv TYPE i"
	default:
		return "USING iv_addr TYPE i iv_val TYPE i"
	}
}

// emitMemoryHelpers emits the helpers as METHODs of the generated class.
func (c *compiler) emitMemoryHelpers() {
	c.line("METHOD mem_zero_pages.")
	c.indent++
	for _, l := range zeroPagesBody() {
		c.line("%s", l)
	}
	c.indent--
	c.line("ENDMETHOD.")
	for _, h := range memHelperDefs {
		c.line("METHOD %s.", h.name)
		c.indent++
		for _, l := range memHelperBody(h, "mv_mem", "mv_mem_pages", memoryLimit(c.mod.Memory), false) {
			c.line("%s", l)
		}
		c.indent--
		c.line("ENDMETHOD.")
	}
}

// emitFUGRMemoryHelpers returns the helpers as FORMs on the function group's
// global memory.
func emitFUGRMemoryHelpers(mod *Module) string {
	var sb strings.Builder
	sb.WriteString("FORM mem_zero_pages USING iv_pages TYPE i CHANGING rv_mem TYPE xstring.\n")
	for _, l := range zeroPagesBody() {
		sb.WriteString("  " + l + "\n")
	}
	sb.WriteString("ENDFORM.\n\n")
	for _, h := range memHelperDefs {
		fmt.Fprintf(&sb, "FORM %s %s.\n", h.name, memHelperParams(h))
		for _, l := range memHelperBody(h, "gv_mem", "gv_mem_pages", memoryLimit(mod.Memory), true) {
			sb.WriteString("  " + l + "\n")
		}
		sb.WriteString("ENDFORM.\n\n")
	}
	return sb.String()
}

// i32 arithmetic is computed in int8 before narrowing to TYPE i: TYPE i would
// raise CX_SY_ARITHMETIC_OVERFLOW on a kernel, and the largest intermediate,
// a product of two i32 values, is at most 2^62, which fits in int8. int8 is
// native 64-bit arithmetic, much cheaper than packed decimal.
var i32HelperDefs = []struct {
	name string
	op   string
}{
	{"i32_add", "+"},
	{"i32_sub", "-"},
	{"i32_mul", "*"},
}

func (c *compiler) emitI32HelperDeclarations(kind string) {
	for _, h := range i32HelperDefs {
		c.line("%s %s IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv) TYPE i.", kind, h.name)
	}
}

func i32HelperBody(op string) []string {
	return []string{
		"DATA lv_p TYPE int8.",
		"lv_p = iv_a.",
		fmt.Sprintf("lv_p = lv_p %s iv_b.", op),
		"lv_p = lv_p MOD 4294967296.",
		"IF lv_p >= 2147483648.",
		"  lv_p = lv_p - 4294967296.",
		"ENDIF.",
		"rv = lv_p.",
	}
}

func (c *compiler) emitI32Helpers() {
	for _, h := range i32HelperDefs {
		c.line("METHOD %s.", h.name)
		c.indent++
		for _, l := range i32HelperBody(h.op) {
			c.line("%s", l)
		}
		c.indent--
		c.line("ENDMETHOD.")
	}
}

func emitFUGRI32Helpers() string {
	var sb strings.Builder
	for _, h := range i32HelperDefs {
		fmt.Fprintf(&sb, "FORM %s USING iv_a TYPE i iv_b TYPE i CHANGING rv TYPE i.\n", h.name)
		for _, l := range i32HelperBody(h.op) {
			sb.WriteString("  " + l + "\n")
		}
		sb.WriteString("ENDFORM.\n\n")
	}
	return sb.String()
}

func (c *compiler) emitI32Call(name, result, a, b string) {
	if c.useFUGR {
		c.line("PERFORM %s USING %s %s CHANGING %s.", name, a, b, result)
	} else if c.useRuntimeI32 {
		c.line("%s = zcl_wasm_rt=>%s( iv_a = %s iv_b = %s ).", result, name, a, b)
	} else {
		c.line("%s = %s( iv_a = %s iv_b = %s ).", result, name, a, b)
	}
}

// emitDataSegments writes the data segments into memory with REPLACE SECTION
// (no offset writes on an xstring), through a typed xstring so the literal is
// read as hex, in chunks that keep each literal under 255 characters.
func (c *compiler) emitDataSegments(mem string) {
	declared := false
	for _, seg := range c.mod.Data {
		// Keep the complete assignment, including its actual indentation, within 255.
		chunk := (255 - len(sourceIndent(c.indent)) - len("lv_seg = ''.")) / 2
		for off := 0; off < len(seg.Data); off += chunk {
			end := min(off+chunk, len(seg.Data))
			if !declared {
				c.line("DATA lv_seg TYPE xstring.")
				declared = true
			}
			c.line("lv_seg = '%s'.", bytesToHex(seg.Data[off:end]))
			c.line("REPLACE SECTION OFFSET %d LENGTH %d OF %s WITH lv_seg IN BYTE MODE.",
				int(seg.Offset)+off, end-off, mem)
		}
	}
}
