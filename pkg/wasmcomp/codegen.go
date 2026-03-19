package wasmcomp

import (
	"fmt"
	"strings"
)

// Compile takes a parsed WASM module and produces ABAP source code.
func Compile(mod *Module, className string) string {
	c := &compiler{
		mod:       mod,
		className: className,
	}
	return c.emit()
}

type compiler struct {
	mod       *Module
	className string
	sb        strings.Builder
	indent    int
}

func (c *compiler) emit() string {
	c.emitDefinition()
	c.line("")
	c.emitImplementation()
	return c.sb.String()
}

// --- Class Definition ---

func (c *compiler) emitDefinition() {
	c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", c.className)
	c.indent++
	c.line("PUBLIC SECTION.")
	c.indent++

	// Constructor
	c.line("METHODS constructor.")

	// Exported functions as public methods
	for _, f := range c.mod.Functions {
		if f.ExportName != "" && f.Type != nil {
			c.emitMethodSignature(f.ExportName, f.Type, true)
		}
	}

	c.indent--
	c.line("PRIVATE SECTION.")
	c.indent++

	// Linear memory
	c.line("DATA mv_mem TYPE xstring.")
	c.line("DATA mv_mem_pages TYPE i.")

	// Globals
	for i, g := range c.mod.Globals {
		c.line("DATA mv_g%d TYPE %s.", i, g.Type.ABAPType())
	}

	// Function table
	for i, elem := range c.mod.Elements {
		_ = elem
		c.line("DATA mt_tab%d TYPE STANDARD TABLE OF i WITH DEFAULT KEY.", i)
	}

	// Memory helper methods
	c.line("METHODS mem_ld_i32 IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_st_i32 IMPORTING iv_addr TYPE i iv_val TYPE i.")
	c.line("METHODS mem_ld_i32_8u IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_ld_i32_8s IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_ld_i32_16u IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_st_i32_8 IMPORTING iv_addr TYPE i iv_val TYPE i.")
	c.line("METHODS mem_st_i32_16 IMPORTING iv_addr TYPE i iv_val TYPE i.")
	c.line("METHODS mem_grow IMPORTING iv_pages TYPE i RETURNING VALUE(rv) TYPE i.")

	// Internal functions (non-exported)
	for i, f := range c.mod.Functions {
		if f.Type != nil {
			name := fmt.Sprintf("f%d", i)
			if f.ExportName != "" {
				name = sanitizeABAP(f.ExportName)
			}
			c.emitMethodSignature(name, f.Type, false)
		}
	}

	c.indent--
	c.indent--
	c.line("ENDCLASS.")
}

func (c *compiler) emitMethodSignature(name string, ft *FuncType, isPublic bool) {
	parts := []string{"METHODS " + sanitizeABAP(name)}

	// Parameters
	if len(ft.Params) > 0 {
		var params []string
		for i, p := range ft.Params {
			params = append(params, fmt.Sprintf("p%d TYPE %s", i, p.ABAPType()))
		}
		parts = append(parts, "IMPORTING "+strings.Join(params, " "))
	}

	// Return value
	if len(ft.Results) == 1 {
		parts = append(parts, fmt.Sprintf("RETURNING VALUE(rv) TYPE %s", ft.Results[0].ABAPType()))
	}

	c.line("%s.", strings.Join(parts, " "))
}

// --- Class Implementation ---

func (c *compiler) emitImplementation() {
	c.line("CLASS %s IMPLEMENTATION.", c.className)
	c.indent++

	// Constructor
	c.emitConstructor()

	// Memory helpers
	c.emitMemoryHelpers()

	// Functions
	for i, f := range c.mod.Functions {
		if f.Type != nil {
			name := fmt.Sprintf("f%d", i)
			if f.ExportName != "" {
				name = sanitizeABAP(f.ExportName)
			}
			c.emitFunction(name, &f)
		}
	}

	c.indent--
	c.line("ENDCLASS.")
}

func (c *compiler) emitConstructor() {
	c.line("METHOD constructor.")
	c.indent++

	// Initialize memory
	if c.mod.Memory != nil {
		pages := c.mod.Memory.Min
		if pages == 0 {
			pages = 1
		}
		c.line("mv_mem_pages = %d.", pages)
		// Initialize memory to zeros
		c.line("DATA(lv_page) = CONV xstring( '00' ).")
		c.line("DO %d TIMES.", pages*65536-1)
		c.indent++
		c.line("CONCATENATE mv_mem lv_page INTO mv_mem IN BYTE MODE.")
		c.indent--
		c.line("ENDDO.")
	}

	// Initialize globals
	for i, g := range c.mod.Globals {
		if g.InitI32 != 0 {
			c.line("mv_g%d = %d.", i, g.InitI32)
		}
		if g.InitI64 != 0 {
			c.line("mv_g%d = %d.", i, g.InitI64)
		}
	}

	// Initialize data segments
	for _, seg := range c.mod.Data {
		if len(seg.Data) > 0 {
			hex := bytesToHex(seg.Data)
			c.line("mv_mem+%d(%d) = '%s'.", seg.Offset, len(seg.Data), hex)
		}
	}

	// Initialize element segments (function tables)
	for i, elem := range c.mod.Elements {
		for _, funcIdx := range elem.FuncIndices {
			c.line("APPEND %d TO mt_tab%d.", funcIdx, i)
		}
	}

	c.indent--
	c.line("ENDMETHOD.")
}

func (c *compiler) emitMemoryHelpers() {
	// i32 load (little-endian)
	c.line("METHOD mem_ld_i32.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 4.")
	c.line("lv_b = mv_mem+iv_addr(4).")
	c.line("\" Little-endian to big-endian")
	c.line("DATA(lv_r) = lv_b+3(1) && lv_b+2(1) && lv_b+1(1) && lv_b+0(1).")
	c.line("rv = lv_r.")
	c.indent--
	c.line("ENDMETHOD.")

	// i32 store (little-endian)
	c.line("METHOD mem_st_i32.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 4.")
	c.line("lv_b = iv_val.")
	c.line("\" Big-endian to little-endian")
	c.line("DATA(lv_r) = lv_b+3(1) && lv_b+2(1) && lv_b+1(1) && lv_b+0(1).")
	c.line("mv_mem+iv_addr(4) = lv_r.")
	c.indent--
	c.line("ENDMETHOD.")

	// i32 load 8-bit unsigned
	c.line("METHOD mem_ld_i32_8u.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 1.")
	c.line("lv_b = mv_mem+iv_addr(1).")
	c.line("rv = lv_b.")
	c.indent--
	c.line("ENDMETHOD.")

	// i32 load 8-bit signed
	c.line("METHOD mem_ld_i32_8s.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 1.")
	c.line("lv_b = mv_mem+iv_addr(1).")
	c.line("rv = lv_b.")
	c.line("IF rv > 127. rv = rv - 256. ENDIF.")
	c.indent--
	c.line("ENDMETHOD.")

	// i32 load 16-bit unsigned
	c.line("METHOD mem_ld_i32_16u.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 2.")
	c.line("lv_b = mv_mem+iv_addr(2).")
	c.line("DATA(lv_r) = lv_b+1(1) && lv_b+0(1).")
	c.line("rv = lv_r.")
	c.indent--
	c.line("ENDMETHOD.")

	// i32 store 8-bit
	c.line("METHOD mem_st_i32_8.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 1.")
	c.line("lv_b = iv_val.")
	c.line("mv_mem+iv_addr(1) = lv_b.")
	c.indent--
	c.line("ENDMETHOD.")

	// i32 store 16-bit
	c.line("METHOD mem_st_i32_16.")
	c.indent++
	c.line("DATA lv_b TYPE x LENGTH 2.")
	c.line("lv_b = iv_val.")
	c.line("DATA(lv_r) = lv_b+1(1) && lv_b+0(1).")
	c.line("mv_mem+iv_addr(2) = lv_r.")
	c.indent--
	c.line("ENDMETHOD.")

	// memory.grow
	c.line("METHOD mem_grow.")
	c.indent++
	c.line("rv = mv_mem_pages.")
	c.line("DATA(lv_new_bytes) = iv_pages * 65536.")
	c.line("DATA lv_zeros TYPE xstring.")
	c.line("DO lv_new_bytes TIMES.")
	c.indent++
	c.line("CONCATENATE lv_zeros '00' INTO lv_zeros IN BYTE MODE.")
	c.indent--
	c.line("ENDDO.")
	c.line("CONCATENATE mv_mem lv_zeros INTO mv_mem IN BYTE MODE.")
	c.line("mv_mem_pages = mv_mem_pages + iv_pages.")
	c.indent--
	c.line("ENDMETHOD.")
}

// --- Function Code Generation ---

func (c *compiler) emitFunction(name string, f *Function) {
	c.line("METHOD %s.", sanitizeABAP(name))
	c.indent++

	// All params + locals as local variables
	totalLocals := len(f.Type.Params) + len(f.Locals)
	for i := 0; i < len(f.Locals); i++ {
		localIdx := len(f.Type.Params) + i
		c.line("DATA l%d TYPE %s.", localIdx, f.Locals[i].ABAPType())
	}

	// Stack variables (we'll need to figure out the max stack depth)
	maxStack := estimateMaxStack(f.Code)
	for i := 0; i < maxStack; i++ {
		c.line("DATA s%d TYPE i.", i) // simplified: all TYPE i for now
	}

	_ = totalLocals

	// Emit instructions
	stack := &virtualStack{}
	c.emitInstructions(f, f.Code, stack, 0)

	c.indent--
	c.line("ENDMETHOD.")
}

func (c *compiler) emitInstructions(f *Function, code []Instruction, stack *virtualStack, blockDepth int) {
	for i := 0; i < len(code); i++ {
		inst := code[i]
		switch inst.Op {

		case OpNop:
			// nothing

		case OpUnreachable:
			c.line("RAISE EXCEPTION TYPE cx_sy_program_error. \" unreachable")

		// Constants
		case OpI32Const:
			v := stack.push()
			c.line("%s = %d.", v, inst.I32Value)
		case OpI64Const:
			v := stack.push()
			c.line("%s = %d.", v, inst.I64Value)
		case OpF32Const:
			v := stack.push()
			c.line("%s = '%f'.", v, inst.F32Value)
		case OpF64Const:
			v := stack.push()
			c.line("%s = '%f'.", v, inst.F64Value)

		// Local/Global access
		case OpLocalGet:
			v := stack.push()
			c.line("%s = %s.", v, c.localName(f, inst.LocalIndex))
		case OpLocalSet:
			v := stack.pop()
			c.line("%s = %s.", c.localName(f, inst.LocalIndex), v)
		case OpLocalTee:
			v := stack.peek()
			c.line("%s = %s.", c.localName(f, inst.LocalIndex), v)
		case OpGlobalGet:
			v := stack.push()
			c.line("%s = mv_g%d.", v, inst.GlobalIndex)
		case OpGlobalSet:
			v := stack.pop()
			c.line("mv_g%d = %s.", inst.GlobalIndex, v)

		// i32 arithmetic
		case OpI32Add:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = %s + %s.", r, a, b)
		case OpI32Sub:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = %s - %s.", r, a, b)
		case OpI32Mul:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = %s * %s.", r, a, b)
		case OpI32DivS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = %s / %s.", r, a, b)
		case OpI32DivU:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>div_u32( iv_a = %s iv_b = %s ).", r, a, b)
		case OpI32RemS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = %s MOD %s.", r, a, b)
		case OpI32RemU:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>rem_u32( iv_a = %s iv_b = %s ).", r, a, b)

		// Bitwise
		case OpI32And:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>and32( iv_a = %s iv_b = %s ).", r, a, b)
		case OpI32Or:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>or32( iv_a = %s iv_b = %s ).", r, a, b)
		case OpI32Xor:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>xor32( iv_a = %s iv_b = %s ).", r, a, b)
		case OpI32Shl:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>shl32( iv_val = %s iv_shift = %s ).", r, a, b)
		case OpI32ShrS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>shr_s32( iv_val = %s iv_shift = %s ).", r, a, b)
		case OpI32ShrU:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>shr_u32( iv_val = %s iv_shift = %s ).", r, a, b)

		// Comparisons
		case OpI32Eqz:
			a := stack.pop()
			r := stack.push()
			c.line("IF %s = 0. %s = 1. ELSE. %s = 0. ENDIF.", a, r, r)
		case OpI32Eq:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s = %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, r, r)
		case OpI32Ne:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s <> %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, r, r)
		case OpI32LtS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s < %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, r, r)
		case OpI32GtS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s > %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, r, r)
		case OpI32LeS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s <= %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, r, r)
		case OpI32GeS:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s >= %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, r, r)
		case OpI32LtU, OpI32GtU, OpI32LeU, OpI32GeU:
			b, a := stack.pop(), stack.pop()
			r := stack.push()
			op := "lt"
			switch inst.Op {
			case OpI32GtU:
				op = "gt"
			case OpI32LeU:
				op = "le"
			case OpI32GeU:
				op = "ge"
			}
			c.line("IF zcl_wasm_rt=>%s_u32( iv_a = %s iv_b = %s ) = abap_true. %s = 1. ELSE. %s = 0. ENDIF.", op, a, b, r, r)

		// Memory
		case OpI32Load:
			addr := stack.pop()
			r := stack.push()
			if inst.Offset > 0 {
				c.line("%s = mem_ld_i32( %s + %d ).", r, addr, inst.Offset)
			} else {
				c.line("%s = mem_ld_i32( %s ).", r, addr)
			}
		case OpI32Store:
			val, addr := stack.pop(), stack.pop()
			if inst.Offset > 0 {
				c.line("mem_st_i32( iv_addr = %s + %d iv_val = %s ).", addr, inst.Offset, val)
			} else {
				c.line("mem_st_i32( iv_addr = %s iv_val = %s ).", addr, val)
			}
		case OpI32Load8U:
			addr := stack.pop()
			r := stack.push()
			if inst.Offset > 0 {
				c.line("%s = mem_ld_i32_8u( %s + %d ).", r, addr, inst.Offset)
			} else {
				c.line("%s = mem_ld_i32_8u( %s ).", r, addr)
			}
		case OpI32Load8S:
			addr := stack.pop()
			r := stack.push()
			if inst.Offset > 0 {
				c.line("%s = mem_ld_i32_8s( %s + %d ).", r, addr, inst.Offset)
			} else {
				c.line("%s = mem_ld_i32_8s( %s ).", r, addr)
			}
		case OpI32Store8:
			val, addr := stack.pop(), stack.pop()
			if inst.Offset > 0 {
				c.line("mem_st_i32_8( iv_addr = %s + %d iv_val = %s ).", addr, inst.Offset, val)
			} else {
				c.line("mem_st_i32_8( iv_addr = %s iv_val = %s ).", addr, val)
			}

		case OpMemorySize:
			r := stack.push()
			c.line("%s = mv_mem_pages.", r)
		case OpMemoryGrow:
			pages := stack.pop()
			r := stack.push()
			c.line("%s = mem_grow( %s ).", r, pages)

		// Call
		case OpCall:
			c.emitCall(f, inst.FuncIndex, stack)

		// Control flow
		case OpBlock:
			c.line("DO 1 TIMES. \" block")
			c.indent++
		case OpLoop:
			c.line("DO. \" loop")
			c.indent++
		case OpIf:
			cond := stack.pop()
			c.line("IF %s <> 0. \" if", cond)
			c.indent++
		case OpElse:
			c.indent--
			c.line("ELSE.")
			c.indent++
		case OpEnd:
			if blockDepth > 0 || c.indent > 2 {
				c.indent--
				// Determine if this ends a DO or IF
				c.line("ENDDO. \" end")
			}
		case OpBr:
			if inst.LabelIndex == 0 {
				c.line("EXIT. \" br 0")
			} else {
				c.line("lv_br = %d. EXIT. \" br %d", inst.LabelIndex, inst.LabelIndex)
			}
		case OpBrIf:
			cond := stack.pop()
			if inst.LabelIndex == 0 {
				c.line("IF %s <> 0. EXIT. ENDIF. \" br_if 0", cond)
			} else {
				c.line("IF %s <> 0. lv_br = %d. EXIT. ENDIF. \" br_if %d", cond, inst.LabelIndex, inst.LabelIndex)
			}
		case OpReturn:
			if len(f.Type.Results) > 0 {
				ret := stack.pop()
				c.line("rv = %s. RETURN.", ret)
			} else {
				c.line("RETURN.")
			}

		// Stack
		case OpDrop:
			stack.pop()
		case OpSelect:
			cond, b, a := stack.pop(), stack.pop(), stack.pop()
			r := stack.push()
			c.line("IF %s <> 0. %s = %s. ELSE. %s = %s. ENDIF.", cond, r, a, r, b)

		// Conversions (simplified)
		case OpI32WrapI64:
			a := stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>wrap_i64( %s ).", r, a)
		case OpI64ExtendI32S:
			a := stack.pop()
			r := stack.push()
			c.line("%s = %s. \" i64.extend_i32_s", r, a)
		case OpI64ExtendI32U:
			a := stack.pop()
			r := stack.push()
			c.line("%s = zcl_wasm_rt=>extend_u32( %s ).", r, a)

		default:
			c.line("\" TODO: opcode 0x%02X", inst.Op)
		}
	}
}

func (c *compiler) emitCall(f *Function, funcIndex int, stack *virtualStack) {
	// Determine if it's an import or local function
	if funcIndex < c.mod.NumImportedFuncs {
		// Import call
		imp := c.findImport(funcIndex)
		if imp != nil {
			c.line("\" IMPORT: %s.%s (TODO)", imp.Module, imp.Name)
		}
		return
	}

	localIdx := funcIndex - c.mod.NumImportedFuncs
	if localIdx >= len(c.mod.Functions) {
		c.line("\" ERROR: invalid function index %d", funcIndex)
		return
	}

	target := &c.mod.Functions[localIdx]
	if target.Type == nil {
		return
	}

	// Pop arguments from stack (in reverse)
	args := make([]string, len(target.Type.Params))
	for i := len(args) - 1; i >= 0; i-- {
		args[i] = stack.pop()
	}

	// Build call
	name := fmt.Sprintf("f%d", localIdx)
	if target.ExportName != "" {
		name = sanitizeABAP(target.ExportName)
	}

	if len(target.Type.Results) > 0 {
		result := stack.push()
		if len(args) > 0 {
			var paramParts []string
			for i, a := range args {
				paramParts = append(paramParts, fmt.Sprintf("p%d = %s", i, a))
			}
			c.line("%s = %s( %s ).", result, name, strings.Join(paramParts, " "))
		} else {
			c.line("%s = %s( ).", result, name)
		}
	} else {
		if len(args) > 0 {
			var paramParts []string
			for i, a := range args {
				paramParts = append(paramParts, fmt.Sprintf("p%d = %s", i, a))
			}
			c.line("%s( %s ).", name, strings.Join(paramParts, " "))
		} else {
			c.line("%s( ).", name)
		}
	}
}

func (c *compiler) findImport(funcIndex int) *Import {
	for i := range c.mod.Imports {
		if c.mod.Imports[i].Kind == 0 && c.mod.Imports[i].FuncIndex == funcIndex {
			return &c.mod.Imports[i]
		}
	}
	return nil
}

func (c *compiler) localName(f *Function, index int) string {
	if index < len(f.Type.Params) {
		return fmt.Sprintf("p%d", index)
	}
	return fmt.Sprintf("l%d", index)
}

// --- Virtual Stack ---

type virtualStack struct {
	depth int
}

func (s *virtualStack) push() string {
	name := fmt.Sprintf("s%d", s.depth)
	s.depth++
	return name
}

func (s *virtualStack) pop() string {
	if s.depth <= 0 {
		return "s0"
	}
	s.depth--
	return fmt.Sprintf("s%d", s.depth)
}

func (s *virtualStack) peek() string {
	if s.depth <= 0 {
		return "s0"
	}
	return fmt.Sprintf("s%d", s.depth-1)
}

// --- Helpers ---

func (c *compiler) line(format string, args ...any) {
	prefix := strings.Repeat("  ", c.indent)
	c.sb.WriteString(prefix)
	fmt.Fprintf(&c.sb, format, args...)
	c.sb.WriteByte('\n')
}

func sanitizeABAP(name string) string {
	// ABAP identifiers: max 30 chars, alphanumeric + underscore
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "$", "_")
	if len(name) > 30 {
		name = name[:30]
	}
	return name
}

func bytesToHex(data []byte) string {
	var sb strings.Builder
	for _, b := range data {
		fmt.Fprintf(&sb, "%02X", b)
	}
	return sb.String()
}

func estimateMaxStack(code []Instruction) int {
	max := 0
	depth := 0
	for _, inst := range code {
		switch inst.Op {
		case OpI32Const, OpI64Const, OpF32Const, OpF64Const,
			OpLocalGet, OpGlobalGet, OpMemorySize:
			depth++
		case OpLocalSet, OpGlobalSet, OpDrop,
			OpI32Store, OpI64Store, OpF32Store, OpF64Store,
			OpI32Store8, OpI32Store16, OpI64Store8, OpI64Store16, OpI64Store32,
			OpBrIf:
			depth--
		case OpI32Add, OpI32Sub, OpI32Mul, OpI32DivS, OpI32DivU,
			OpI32RemS, OpI32RemU, OpI32And, OpI32Or, OpI32Xor,
			OpI32Shl, OpI32ShrS, OpI32ShrU,
			OpI32Eq, OpI32Ne, OpI32LtS, OpI32LtU, OpI32GtS, OpI32GtU,
			OpI32LeS, OpI32LeU, OpI32GeS, OpI32GeU:
			depth-- // pop 2, push 1
		case OpSelect:
			depth -= 2 // pop 3, push 1
		}
		if depth < 0 {
			depth = 0
		}
		if depth > max {
			max = depth
		}
	}
	if max < 8 {
		max = 8
	}
	return max
}
