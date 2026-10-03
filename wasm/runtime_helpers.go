package wasm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Runtime method templates come from the shared runtime class. Keeping the
// signatures and bodies in one source prevents the single class from drifting
// away from the multi-class backend.
var runtimeDeclarationRE = regexp.MustCompile(`(?m)^\s*CLASS-METHODS ([a-z0-9_]+) (.+)\.$`)
var runtimeMethodRE = regexp.MustCompile(`(?s)\bMETHOD ([a-z0-9_]+)\.(.*?)\bENDMETHOD\.`)
var inlineDataRE = regexp.MustCompile(`DATA\(([a-z][a-z0-9_]*)\)\s*=`)
var simpleConvRE = regexp.MustCompile(`CONV\s+(?:int8|i|f|xstring)\(\s*([^()]*)\s*\)`)
var xsdboolRE = regexp.MustCompile(`rv\s*=\s*xsdbool\(\s*([^()]*)\s*\)\.`)

func runtimeTemplates() (map[string]string, map[string]string) {
	src := emitRuntimeClass()
	declarations := make(map[string]string)
	bodies := make(map[string]string)
	for _, m := range runtimeDeclarationRE.FindAllStringSubmatch(src, -1) {
		declarations[m[1]] = "METHODS " + m[1] + " " + m[2] + "."
	}
	for _, m := range runtimeMethodRE.FindAllStringSubmatch(src, -1) {
		bodies[m[1]] = strings.TrimSpace(m[2])
	}
	return declarations, bodies
}

// Single-class memory helpers operate on the owning instance's buffer. Passing
// that buffer by reference prevents runtimes from updating it in place.
func singleClassRuntimeTemplates() (map[string]string, map[string]string) {
	declarations, bodies := runtimeTemplates()
	for name, declaration := range declarations {
		declaration = strings.ReplaceAll(declaration, "iv_mem TYPE xstring ", "")
		declaration = strings.ReplaceAll(declaration, " CHANGING cv_mem TYPE xstring", "")
		declarations[name] = declaration
		bodies[name] = runtimeMemoryRE.ReplaceAllString(bodies[name], "mv_mem")
	}
	return declarations, bodies
}

var runtimeMemoryRE = regexp.MustCompile(`\b(?:iv_mem|cv_mem)\b`)

func legacyRuntimeBody(name, body string) string {
	// The shared runtime predates the v702 class backend. Downport its
	// inline declarations and elementary conversions for every backend.
	declarations := []string{}
	seen := make(map[string]bool)
	for _, match := range inlineDataRE.FindAllStringSubmatch(body, -1) {
		variable := match[1]
		if seen[variable] {
			continue
		}
		seen[variable] = true
		typ := "int8"
		switch variable {
		case "lv_shift", "lv_chunks", "lv_remainder", "lv_bytes", "lv_off":
			typ = "i"
		case "lv_src_data":
			typ = "xstring"
		case "lv_r":
			if strings.Contains(name, "mem_") || strings.HasSuffix(name, "64") && (strings.HasPrefix(name, "and") || strings.HasPrefix(name, "or") || strings.HasPrefix(name, "xor")) {
				typ = "x LENGTH 8"
			}
			if name == "mem_ld_i32_16s" {
				typ = "x LENGTH 2"
			}
		}
		declarations = append(declarations, "DATA "+variable+" TYPE "+typ+".")
	}
	body = inlineDataRE.ReplaceAllString(body, "$1 =")
	for simpleConvRE.MatchString(body) {
		body = simpleConvRE.ReplaceAllString(body, "$1")
	}
	body = xsdboolRE.ReplaceAllString(body, "IF $1. rv = abap_true. ELSE. rv = abap_false. ENDIF.")
	if len(declarations) > 0 {
		body = strings.Join(declarations, "\n") + "\n" + body
	}
	return body
}

// i64AddSubBody splits each signed operand with floor DIV and non-negative
// MOD. The low sum/difference is within +/-2^33, the high sum within +/-2^32.
// Normalizing the high half before recombination keeps even INT64_MIN safe.
func i64AddSubBody(op string) string {
	return `DATA lv_al TYPE int8.
DATA lv_bl TYPE int8.
DATA lv_ah TYPE int8.
DATA lv_bh TYPE int8.
DATA lv_lo TYPE int8.
DATA lv_hi TYPE int8.
lv_al = iv_a MOD 4294967296.
lv_bl = iv_b MOD 4294967296.
lv_ah = iv_a DIV 4294967296.
lv_bh = iv_b DIV 4294967296.
lv_lo = lv_al ` + op + ` lv_bl.
lv_hi = lv_ah ` + op + ` lv_bh.
lv_hi = lv_hi + lv_lo DIV 4294967296.
lv_lo = lv_lo MOD 4294967296.
lv_hi = lv_hi MOD 4294967296.
IF lv_hi >= 2147483648.
lv_hi = lv_hi - 4294967296.
ENDIF.
rv = lv_hi * 4294967296 + lv_lo.`
}

// i64MulBody performs base-2^16 convolution, discarding limbs above bit 63.
// The largest sum is four products of 65535 plus a carry, below 2^34.
// Extracting via DIV also handles negative inputs without negating INT64_MIN.
func i64MulBody() string {
	var lines []string
	for _, name := range []string{"a", "b", "a0", "a1", "a2", "a3", "b0", "b1", "b2", "b3", "t", "r0", "r1", "r2", "r3", "lo", "hi"} {
		lines = append(lines, "DATA lv_"+name+" TYPE int8.")
	}
	for _, operand := range []string{"a", "b"} {
		lines = append(lines, "lv_"+operand+" = iv_"+operand+".")
		for limb := 0; limb < 4; limb++ {
			lines = append(lines, fmt.Sprintf("lv_%s%d = lv_%s MOD 65536.", operand, limb, operand))
			if limb < 3 {
				lines = append(lines, "lv_"+operand+" = lv_"+operand+" DIV 65536.")
			}
		}
	}
	for limb := 0; limb < 4; limb++ {
		if limb == 0 {
			lines = append(lines, "lv_t = lv_a0 * lv_b0.")
		} else {
			lines = append(lines, "lv_t = lv_t DIV 65536.")
			for a := 0; a <= limb; a++ {
				lines = append(lines, fmt.Sprintf("lv_t = lv_t + lv_a%d * lv_b%d.", a, limb-a))
			}
		}
		lines = append(lines, fmt.Sprintf("lv_r%d = lv_t MOD 65536.", limb))
	}
	lines = append(lines,
		"IF lv_r3 >= 32768.",
		"lv_r3 = lv_r3 - 65536.",
		"ENDIF.",
		"lv_lo = lv_r1 * 65536 + lv_r0.",
		"lv_hi = lv_r3 * 65536 + lv_r2.",
		"rv = lv_hi * 4294967296 + lv_lo.")
	return strings.Join(lines, "\n")
}

func i64HelperBody(name string) string {
	switch name {
	case "i64_add":
		return i64AddSubBody("+")
	case "i64_sub":
		return i64AddSubBody("-")
	default:
		return i64MulBody()
	}
}

var i64HelperOps = []struct {
	name string
	op   byte
}{
	{"i64_add", OpI64Add},
	{"i64_sub", OpI64Sub},
	{"i64_mul", OpI64Mul},
}

func emitI64RuntimeMethods() string {
	var sb strings.Builder
	for _, h := range i64HelperOps {
		fmt.Fprintf(&sb, "  METHOD %s.\n", h.name)
		for _, line := range strings.Split(i64HelperBody(h.name), "\n") {
			sb.WriteString("    " + line + "\n")
		}
		sb.WriteString("  ENDMETHOD.\n")
	}
	return sb.String()
}

func emitFUGRI64Helpers(mod *Module) string {
	used := make(map[byte]bool)
	for _, f := range mod.Functions {
		for _, inst := range f.Code {
			used[inst.Op] = true
		}
	}
	var sb strings.Builder
	for _, h := range i64HelperOps {
		if !used[h.op] {
			continue
		}
		fmt.Fprintf(&sb, "FORM %s USING iv_a TYPE int8 iv_b TYPE int8 CHANGING rv TYPE int8.\n", h.name)
		for _, line := range strings.Split(i64HelperBody(h.name), "\n") {
			sb.WriteString("  " + line + "\n")
		}
		sb.WriteString("ENDFORM.\n\n")
	}
	return sb.String()
}

func (c *compiler) emitI64Call(name, result, a, b string) {
	if c.useFUGR {
		c.line("PERFORM %s USING %s %s CHANGING %s.", name, a, b, result)
	} else {
		c.line("%s = zcl_wasm_rt=>%s( iv_a = %s iv_b = %s ).", result, name, a, b)
	}
}

// Repeated division avoids constructing 2^31 or 2^63 in the result type.
func shiftRightSignedBody(is64 bool) string {
	bits := "32"
	if is64 {
		bits = "64"
	}
	return `DATA lv_val TYPE int8.
DATA lv_shift TYPE i.
lv_shift = iv_shift MOD ` + bits + `.
lv_val = iv_val.
DO lv_shift TIMES.
lv_val = lv_val DIV 2.
ENDDO.
rv = lv_val.`
}

func shift64Body(left bool) string {
	common := `DATA lv_bytes TYPE x LENGTH 8.
DATA lv_byte TYPE x LENGTH 1.
DATA lv_new TYPE x LENGTH 1.
DATA lv_shift TYPE i.
DATA lv_off TYPE i.
DATA lv_num TYPE i.
DATA lv_carry TYPE i.
lv_bytes = iv_val.
lv_shift = iv_shift MOD 64.
DO lv_shift TIMES.
lv_carry = 0.
DO 8 TIMES.
`
	if left {
		return common + `lv_off = 8 - sy-index.
lv_byte = lv_bytes+lv_off(1).
lv_num = lv_byte * 2 + lv_carry.
lv_new = lv_num MOD 256.
lv_carry = lv_num DIV 256.
lv_bytes+lv_off(1) = lv_new.
ENDDO.
ENDDO.
rv = lv_bytes.`
	}
	return common + `lv_off = sy-index - 1.
lv_byte = lv_bytes+lv_off(1).
lv_num = lv_byte.
lv_new = lv_num DIV 2 + lv_carry.
lv_carry = lv_num MOD 2 * 128.
lv_bytes+lv_off(1) = lv_new.
ENDDO.
ENDDO.
rv = lv_bytes.`
}

func rotateBody(name string) string {
	bits := "32"
	typeName := "i"
	modulus := "32"
	if strings.HasSuffix(name, "64") {
		bits, typeName, modulus = "64", "int8", "64"
	}
	first, second := "shl", "shr_u"
	if strings.HasPrefix(name, "rotr") {
		first, second = second, first
	}
	return "DATA lv_shift TYPE " + typeName + ".\nDATA lv_other TYPE " + typeName + ".\nDATA lv_a TYPE " + typeName + ".\nDATA lv_b TYPE " + typeName + ".\n" +
		"lv_shift = iv_shift MOD " + modulus + ".\n" +
		"lv_other = " + modulus + " - lv_shift.\n" +
		"lv_a = " + first + bits + "( iv_val = iv_val iv_shift = lv_shift ).\n" +
		"lv_b = " + second + bits + "( iv_val = iv_val iv_shift = lv_other ).\n" +
		"rv = or" + bits + "( iv_a = lv_a iv_b = lv_b )."
}

func (c *compiler) runtimeNames() []string {
	for _, pair := range [][2]string{
		{"rotl32", "shl32"}, {"rotl32", "shr_u32"}, {"rotl32", "or32"},
		{"rotr32", "shl32"}, {"rotr32", "shr_u32"}, {"rotr32", "or32"},
		{"rotl64", "shl64"}, {"rotl64", "shr_u64"}, {"rotl64", "or64"},
		{"rotr64", "shl64"}, {"rotr64", "shr_u64"}, {"rotr64", "or64"},
	} {
		if c.usedRuntime[pair[0]] {
			c.usedRuntime[pair[1]] = true
		}
	}
	names := make([]string, 0, len(c.usedRuntime))
	for name := range c.usedRuntime {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *compiler) emitRuntimeDeclarations() {
	declarations, _ := singleClassRuntimeTemplates()
	for _, name := range c.runtimeNames() {
		if declaration, ok := declarations[name]; ok {
			c.line("%s", declaration)
		}
	}
}

func (c *compiler) emitRuntimeHelpers() {
	_, bodies := singleClassRuntimeTemplates()
	for _, name := range c.runtimeNames() {
		body, ok := bodies[name]
		if !ok {
			continue
		}
		c.line("METHOD %s.", name)
		c.indent++
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				c.line("%s", line)
			}
		}
		c.indent--
		c.line("ENDMETHOD.")
	}
}

// ABAP comments are a * in column 1 or start at a double quote outside
// single-quoted literals.
func stripABAPComment(line string) string {
	// A full-line comment has * in column 1 only; an indented * is code,
	// e.g. a wrapped continuation line starting with a multiplication.
	if strings.HasPrefix(line, "*") {
		return ""
	}
	quoted := false
	for i := 0; i < len(line); i++ {
		if line[i] == '\'' {
			if quoted && i+1 < len(line) && line[i+1] == '\'' {
				i++
				continue
			}
			quoted = !quoted
		} else if line[i] == '"' && !quoted {
			return strings.TrimRight(line[:i], " \t")
		}
	}
	return line
}

func stripABAPComments(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		line = stripABAPComment(line)
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n") + "\n"
}
