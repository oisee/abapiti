// Package llvm2abap compiles LLVM IR text format to idiomatic typed ABAP.
package llvm2abap

import (
	"fmt"
	"regexp"
	"strings"
)

// Module represents a parsed LLVM IR module.
type Module struct {
	SourceFile string
	Types      map[string]*StructType // named struct types
	Functions  []*Function
	Globals    []*Global
}

// StructType represents an LLVM named struct type.
type StructType struct {
	Name   string
	Fields []Type
}

// Type represents an LLVM IR type.
type Type struct {
	Kind     TypeKind
	BitWidth int    // for IntType
	Name     string // for StructType (named)
	Elem     *Type  // for PtrType, ArrayType
	Len      int    // for ArrayType
}

type TypeKind int

const (
	VoidType TypeKind = iota
	IntType
	FloatType
	DoubleType
	PtrType
	StructTypeKind
	ArrayTypeKind
)

func (t Type) ABAPType() string {
	switch t.Kind {
	case IntType:
		switch {
		case t.BitWidth <= 32:
			return "i"
		case t.BitWidth <= 64:
			return "int8"
		default:
			return "i"
		}
	case FloatType, DoubleType:
		return "f"
	case PtrType:
		return "i" // pointer = offset
	case VoidType:
		return ""
	default:
		return "i"
	}
}

// Function represents an LLVM IR function.
type Function struct {
	Name       string
	ReturnType Type
	Params     []Param
	Blocks     []*BasicBlock
	IsExternal bool // declare (no body)
}

// Param represents a function parameter.
type Param struct {
	Name string // %0, %1, etc.
	Type Type
}

// BasicBlock represents an LLVM basic block.
type BasicBlock struct {
	Label string
	Insts []*Instruction
}

// Instruction represents a single LLVM IR instruction.
type Instruction struct {
	Result string // destination register (%3, etc.)
	Op     string // add, sub, mul, icmp, br, ret, ...
	Type   Type
	Args   []string
	// For icmp
	Predicate string // eq, ne, slt, sgt, sle, sge, ult, ugt
	// For br
	Cond      string
	TrueLabel string
	FalseLabel string
	// For phi
	PhiPairs []PhiPair
	// For call
	CallTarget string
	// For select
	SelectCond string
	SelectTrue string
	SelectFalse string
}

type PhiPair struct {
	Value string
	Label string
}

// Global represents an LLVM global variable.
type Global struct {
	Name string
	Type Type
	Init string
}

// Parse parses LLVM IR text format into a Module.
func Parse(source string) (*Module, error) {
	m := &Module{
		Types: make(map[string]*StructType),
	}

	lines := strings.Split(source, "\n")
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "!") ||
			strings.HasPrefix(line, "source_filename") || strings.HasPrefix(line, "target") ||
			strings.HasPrefix(line, "attributes") {
			i++
			continue
		}

		// Named struct type: %struct.Point = type { i32, i32 }
		if strings.HasPrefix(line, "%") && strings.Contains(line, "= type {") {
			m.parseStructType(line)
			i++
			continue
		}

		// Function definition
		if strings.HasPrefix(line, "define ") {
			fn, end := m.parseFunction(lines, i)
			if fn != nil {
				m.Functions = append(m.Functions, fn)
			}
			i = end + 1
			continue
		}

		// Function declaration (external)
		if strings.HasPrefix(line, "declare ") {
			fn := m.parseDeclare(line)
			if fn != nil {
				m.Functions = append(m.Functions, fn)
			}
			i++
			continue
		}

		i++
	}

	return m, nil
}

func (m *Module) parseStructType(line string) {
	// %struct.Point = type { i32, i32 }
	re := regexp.MustCompile(`%(\S+)\s*=\s*type\s*\{([^}]*)\}`)
	match := re.FindStringSubmatch(line)
	if match == nil {
		return
	}
	name := match[1]
	fieldsStr := strings.TrimSpace(match[2])
	var fields []Type
	if fieldsStr != "" {
		for _, f := range strings.Split(fieldsStr, ",") {
			fields = append(fields, parseType(strings.TrimSpace(f)))
		}
	}
	m.Types[name] = &StructType{Name: name, Fields: fields}
}

var funcDefRe = regexp.MustCompile(`define\s+(?:\w+\s+)*(\S+)\s+@(\w+)\(([^)]*)\)`)

func (m *Module) parseFunction(lines []string, start int) (*Function, int) {
	line := lines[start]

	match := funcDefRe.FindStringSubmatch(line)
	if match == nil {
		return nil, start
	}

	retType := parseType(match[1])
	name := match[2]
	paramsStr := match[3]

	fn := &Function{
		Name:       name,
		ReturnType: retType,
	}

	// Parse parameters
	if paramsStr != "" {
		for idx, p := range strings.Split(paramsStr, ",") {
			p = strings.TrimSpace(p)
			parts := strings.Fields(p)
			if len(parts) >= 1 {
				typ := parseType(parts[0])
				pname := fmt.Sprintf("%%%d", idx)
				if len(parts) >= 2 {
					last := parts[len(parts)-1]
					if strings.HasPrefix(last, "%") {
						pname = last
					}
				}
				fn.Params = append(fn.Params, Param{Name: pname, Type: typ})
			}
		}
	}

	// Find opening { and parse basic blocks until closing }
	i := start
	// The { might be on the same line as define or the next line
	if !strings.HasSuffix(strings.TrimSpace(line), "{") {
		i++
	}

	var currentBlock *BasicBlock
	for i++; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if line == "}" {
			if currentBlock != nil {
				fn.Blocks = append(fn.Blocks, currentBlock)
			}
			return fn, i
		}

		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Basic block label: "3:" or "entry:" or "3:    ; preds = ..."
		if !strings.HasPrefix(line, "%") && !strings.HasPrefix(line, "ret") &&
			!strings.HasPrefix(line, "br") && !strings.HasPrefix(line, "store") &&
			!strings.HasPrefix(line, "call") && !strings.HasPrefix(line, "tail") &&
			!strings.HasPrefix(line, "switch") &&
			strings.Contains(line, ":") {
			if currentBlock != nil {
				fn.Blocks = append(fn.Blocks, currentBlock)
			}
			label := strings.Split(line, ":")[0]
			label = strings.TrimSpace(label)
			currentBlock = &BasicBlock{Label: label}
			continue
		}

		// First instruction without a label → implicit entry block
		if currentBlock == nil {
			currentBlock = &BasicBlock{Label: "entry"}
		}

		inst := parseInstruction(line)
		if inst != nil {
			currentBlock.Insts = append(currentBlock.Insts, inst)
		}
	}

	return fn, i
}

func (m *Module) parseDeclare(line string) *Function {
	re := regexp.MustCompile(`declare\s+(?:\w+\s+)*(\S+)\s+@(\w+)\(`)
	match := re.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	return &Function{
		Name:       match[2],
		ReturnType: parseType(match[1]),
		IsExternal: true,
	}
}

func parseType(s string) Type {
	s = strings.TrimSpace(s)
	// Remove qualifiers
	for _, q := range []string{"noundef", "nonnull", "returned", "dso_local", "nsw", "nuw", "local_unnamed_addr"} {
		s = strings.TrimSpace(strings.ReplaceAll(s, q, ""))
	}
	s = strings.TrimSpace(s)

	switch s {
	case "void":
		return Type{Kind: VoidType}
	case "i1":
		return Type{Kind: IntType, BitWidth: 1}
	case "i8":
		return Type{Kind: IntType, BitWidth: 8}
	case "i16":
		return Type{Kind: IntType, BitWidth: 16}
	case "i32":
		return Type{Kind: IntType, BitWidth: 32}
	case "i33":
		return Type{Kind: IntType, BitWidth: 33}
	case "i64":
		return Type{Kind: IntType, BitWidth: 64}
	case "float":
		return Type{Kind: FloatType}
	case "double":
		return Type{Kind: DoubleType}
	case "ptr":
		return Type{Kind: PtrType}
	}
	if strings.HasPrefix(s, "%struct.") || strings.HasPrefix(s, "%") {
		return Type{Kind: StructTypeKind, Name: strings.TrimPrefix(s, "%")}
	}
	return Type{Kind: IntType, BitWidth: 32} // fallback
}

func parseInstruction(line string) *Instruction {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, ";") {
		return nil
	}

	inst := &Instruction{}

	// ret instruction
	if strings.HasPrefix(line, "ret ") {
		inst.Op = "ret"
		rest := strings.TrimPrefix(line, "ret ")
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			inst.Type = parseType(parts[0])
			inst.Args = []string{parts[1]}
		} else if len(parts) == 1 && parts[0] == "void" {
			inst.Type = Type{Kind: VoidType}
		}
		return inst
	}

	// br instruction
	if strings.HasPrefix(line, "br ") {
		inst.Op = "br"
		rest := strings.TrimPrefix(line, "br ")
		if strings.HasPrefix(rest, "label %") {
			// Unconditional: br label %target
			inst.TrueLabel = extractLabel(rest)
		} else if strings.HasPrefix(rest, "i1 ") {
			// Conditional: br i1 %cond, label %true, label %false
			parts := strings.SplitN(rest, ",", 3)
			if len(parts) >= 3 {
				inst.Cond = strings.TrimSpace(strings.TrimPrefix(parts[0], "i1 "))
				inst.TrueLabel = extractLabel(parts[1])
				inst.FalseLabel = extractLabel(parts[2])
			}
		}
		return inst
	}

	// Assignment: %result = op ...
	if strings.HasPrefix(line, "%") && strings.Contains(line, " = ") {
		eqIdx := strings.Index(line, " = ")
		inst.Result = line[:eqIdx]
		rest := line[eqIdx+3:]

		// Strip optional keywords
		rest = stripKeywords(rest, []string{"nsw", "nuw", "tail", "nnan", "ninf"})

		// phi instruction
		if strings.HasPrefix(rest, "phi ") {
			inst.Op = "phi"
			re := regexp.MustCompile(`\[\s*(%?\w+)\s*,\s*%(\w+)\s*\]`)
			matches := re.FindAllStringSubmatch(rest, -1)
			for _, m := range matches {
				inst.PhiPairs = append(inst.PhiPairs, PhiPair{Value: m[1], Label: m[2]})
			}
			// Parse type
			parts := strings.Fields(rest)
			if len(parts) >= 2 {
				inst.Type = parseType(parts[1])
			}
			return inst
		}

		// icmp instruction
		if strings.HasPrefix(rest, "icmp ") {
			inst.Op = "icmp"
			parts := strings.Fields(rest)
			// icmp pred type %a, %b
			if len(parts) >= 5 {
				inst.Predicate = parts[1]
				inst.Type = parseType(parts[2])
				inst.Args = []string{
					strings.TrimSuffix(parts[3], ","),
					parts[4],
				}
			}
			return inst
		}

		// select instruction
		if strings.HasPrefix(rest, "select ") {
			inst.Op = "select"
			// select i1 %cond, i32 %true, i32 %false
			re := regexp.MustCompile(`select\s+i1\s+(%\w+)\s*,\s*(\w+)\s+(%\w+)\s*,\s*(\w+)\s+(%?\w+)`)
			m := re.FindStringSubmatch(rest)
			if m != nil {
				inst.SelectCond = m[1]
				inst.Type = parseType(m[2])
				inst.SelectTrue = m[3]
				inst.SelectFalse = m[5]
			}
			return inst
		}

		// call instruction
		if strings.Contains(rest, "call ") {
			inst.Op = "call"
			re := regexp.MustCompile(`call\s+\S+\s+@(\w+)\(([^)]*)\)`)
			m := re.FindStringSubmatch(rest)
			if m != nil {
				inst.CallTarget = m[1]
				inst.Type = parseType(strings.Fields(rest)[1])
				// Parse call args
				if m[2] != "" {
					for _, arg := range strings.Split(m[2], ",") {
						parts := strings.Fields(strings.TrimSpace(arg))
						if len(parts) >= 2 {
							inst.Args = append(inst.Args, parts[len(parts)-1])
						}
					}
				}
			}
			return inst
		}

		// Binary ops: add, sub, mul, sdiv, srem, and, or, xor, shl, ashr, lshr
		// Unary: sub 0, %x (negate)
		// Float: fadd, fsub, fmul, fdiv
		parts := strings.Fields(rest)
		if len(parts) >= 4 {
			inst.Op = parts[0]
			inst.Type = parseType(parts[1])
			inst.Args = []string{
				strings.TrimSuffix(parts[2], ","),
				strings.TrimSuffix(parts[3], ","),
			}
			return inst
		}
		if len(parts) >= 3 {
			inst.Op = parts[0]
			inst.Type = parseType(parts[1])
			inst.Args = []string{strings.TrimSuffix(parts[2], ",")}
			return inst
		}
	}

	return nil
}

func extractLabel(s string) string {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`label\s+%(\w+)`)
	m := re.FindStringSubmatch(s)
	if m != nil {
		return m[1]
	}
	return ""
}

func stripKeywords(s string, kws []string) string {
	for _, kw := range kws {
		s = strings.ReplaceAll(s, " "+kw+" ", " ")
		s = strings.ReplaceAll(s, " "+kw+",", ",")
	}
	return strings.TrimSpace(s)
}

// Compile converts a parsed LLVM IR Module to ABAP source.
func Compile(mod *Module, className string) string {
	c := &abapCompiler{mod: mod, className: className}
	return c.emit()
}

type abapCompiler struct {
	mod       *Module
	className string
	sb        strings.Builder
	indent    int
	// Track which SSA values are function params (for naming)
	paramMap map[string]string // %0 → "a", %1 → "b"
}

func (c *abapCompiler) emit() string {
	c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", c.className)
	c.indent++
	c.line("PUBLIC SECTION.")
	c.indent++

	// Emit typed method signatures for each non-external function
	for _, fn := range c.mod.Functions {
		if fn.IsExternal || strings.HasPrefix(fn.Name, "llvm.") {
			continue
		}
		c.emitMethodDecl(fn)
	}

	c.indent--
	c.indent--
	c.line("ENDCLASS.")
	c.line("")

	c.line("CLASS %s IMPLEMENTATION.", c.className)
	c.indent++

	for _, fn := range c.mod.Functions {
		if fn.IsExternal || strings.HasPrefix(fn.Name, "llvm.") {
			continue
		}
		c.emitMethod(fn)
	}

	c.indent--
	c.line("ENDCLASS.")

	return c.sb.String()
}

func (c *abapCompiler) emitMethodDecl(fn *Function) {
	parts := []string{"CLASS-METHODS " + sanitizeName(fn.Name)}

	if len(fn.Params) > 0 {
		var params []string
		for i, p := range fn.Params {
			name := paramName(i, fn.Name)
			params = append(params, fmt.Sprintf("%s TYPE %s", name, p.Type.ABAPType()))
		}
		parts = append(parts, "IMPORTING "+strings.Join(params, " "))
	}

	if fn.ReturnType.Kind != VoidType {
		parts = append(parts, fmt.Sprintf("RETURNING VALUE(rv) TYPE %s", fn.ReturnType.ABAPType()))
	}

	c.line("%s.", strings.Join(parts, " "))
}

func (c *abapCompiler) emitMethod(fn *Function) {
	c.line("METHOD %s.", sanitizeName(fn.Name))
	c.indent++

	// Build param map
	c.paramMap = make(map[string]string)
	for i, p := range fn.Params {
		c.paramMap[p.Name] = paramName(i, fn.Name)
	}

	// Collect all SSA values used (for DATA declarations)
	vars := c.collectVars(fn)
	if len(vars) > 0 {
		var decls []string
		for _, v := range vars {
			decls = append(decls, fmt.Sprintf("%s TYPE %s", v.name, v.typ))
		}
		c.line("DATA: %s.", strings.Join(decls, ", "))
	}

	// Simple functions (single block, no phi, no complex CFG)
	if len(fn.Blocks) == 1 {
		c.emitBlock(fn.Blocks[0], fn)
	} else {
		// Multi-block: use dispatcher
		c.emitWithDispatcher(fn)
	}

	c.indent--
	c.line("ENDMETHOD.")
	c.line("")
}

type varDecl struct {
	name string
	typ  string
}

func (c *abapCompiler) collectVars(fn *Function) []varDecl {
	seen := make(map[string]bool)
	var result []varDecl

	for _, b := range fn.Blocks {
		for _, inst := range b.Insts {
			if inst.Result != "" {
				name := c.ssaName(inst.Result)
				if !seen[name] && !c.isParam(inst.Result) {
					seen[name] = true
					result = append(result, varDecl{name: name, typ: inst.Type.ABAPType()})
				}
			}
		}
	}
	return result
}

func (c *abapCompiler) emitBlock(block *BasicBlock, fn *Function) {
	for _, inst := range block.Insts {
		c.emitInst(inst, fn)
	}
}

func (c *abapCompiler) emitWithDispatcher(fn *Function) {
	// Emit phi variable assignments at block transitions
	// Use CASE-based dispatcher for multi-block functions
	c.line("DATA lv_block TYPE string VALUE '%s'.", fn.Blocks[0].Label)
	c.line("DO.")
	c.indent++
	c.line("CASE lv_block.")
	c.indent++

	for _, block := range fn.Blocks {
		c.line("WHEN '%s'.", block.Label)
		c.indent++
		c.emitBlock(block, fn)
		c.indent--
	}

	c.indent--
	c.line("ENDCASE.")
	c.indent--
	c.line("ENDDO.")
}

func (c *abapCompiler) emitInst(inst *Instruction, fn *Function) {
	dst := c.ssaName(inst.Result)

	switch inst.Op {
	// Arithmetic
	case "add", "sub", "mul", "fadd", "fsub", "fmul":
		op := map[string]string{"add": "+", "sub": "-", "mul": "*", "fadd": "+", "fsub": "-", "fmul": "*"}[inst.Op]
		c.line("%s = %s %s %s.", dst, c.val(inst.Args[0]), op, c.val(inst.Args[1]))

	case "sdiv", "udiv", "fdiv":
		c.line("%s = %s / %s.", dst, c.val(inst.Args[0]), c.val(inst.Args[1]))

	case "srem", "urem":
		c.line("%s = %s MOD %s.", dst, c.val(inst.Args[0]), c.val(inst.Args[1]))

	// Bitwise
	case "and":
		c.line("DATA(lx_a_%s) = CONV x4( %s ). DATA(lx_b_%s) = CONV x4( %s ). %s = lx_a_%s BIT-AND lx_b_%s.",
			dst, c.val(inst.Args[0]), dst, c.val(inst.Args[1]), dst, dst, dst)
	case "or":
		c.line("DATA(lx_a_%s) = CONV x4( %s ). DATA(lx_b_%s) = CONV x4( %s ). %s = lx_a_%s BIT-OR lx_b_%s.",
			dst, c.val(inst.Args[0]), dst, c.val(inst.Args[1]), dst, dst, dst)
	case "xor":
		c.line("DATA(lx_a_%s) = CONV x4( %s ). DATA(lx_b_%s) = CONV x4( %s ). %s = lx_a_%s BIT-XOR lx_b_%s.",
			dst, c.val(inst.Args[0]), dst, c.val(inst.Args[1]), dst, dst, dst)
	case "shl":
		c.line("TRY. %s = %s * ipow( base = 2 exp = %s ). CATCH cx_root. %s = 0. ENDTRY.",
			dst, c.val(inst.Args[0]), c.val(inst.Args[1]), dst)
	case "ashr":
		c.line("TRY. %s = %s / ipow( base = 2 exp = %s ). CATCH cx_root. %s = 0. ENDTRY.",
			dst, c.val(inst.Args[0]), c.val(inst.Args[1]), dst)
	case "lshr":
		c.line("TRY. %s = %s / ipow( base = 2 exp = %s ). CATCH cx_root. %s = 0. ENDTRY.",
			dst, c.val(inst.Args[0]), c.val(inst.Args[1]), dst)

	// Comparisons
	case "icmp":
		a, b := c.val(inst.Args[0]), c.val(inst.Args[1])
		switch inst.Predicate {
		case "eq":
			c.line("IF %s = %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, dst, dst)
		case "ne":
			c.line("IF %s <> %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, dst, dst)
		case "slt", "ult":
			c.line("IF %s < %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, dst, dst)
		case "sgt", "ugt":
			c.line("IF %s > %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, dst, dst)
		case "sle", "ule":
			c.line("IF %s <= %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, dst, dst)
		case "sge", "uge":
			c.line("IF %s >= %s. %s = 1. ELSE. %s = 0. ENDIF.", a, b, dst, dst)
		default:
			c.line("\" TODO: icmp %s", inst.Predicate)
		}

	// Select
	case "select":
		c.line("IF %s <> 0. %s = %s. ELSE. %s = %s. ENDIF.",
			c.val(inst.SelectCond), dst, c.val(inst.SelectTrue), dst, c.val(inst.SelectFalse))

	// Phi — resolved at branch source (emit assignment to phi var)
	case "phi":
		// PHI nodes handled in dispatcher: assignment before branch
		c.line("\" phi: %s (resolved at branch source)", dst)

	// Branch
	case "br":
		if inst.Cond != "" {
			// Conditional branch
			c.line("IF %s <> 0. lv_block = '%s'. ELSE. lv_block = '%s'. ENDIF.",
				c.val(inst.Cond), inst.TrueLabel, inst.FalseLabel)
		} else if inst.TrueLabel != "" {
			// Unconditional branch
			c.line("lv_block = '%s'.", inst.TrueLabel)
		}

	// Return
	case "ret":
		if len(inst.Args) > 0 && inst.Type.Kind != VoidType {
			c.line("rv = %s. RETURN.", c.val(inst.Args[0]))
		} else {
			c.line("RETURN.")
		}

	// Call
	case "call":
		target := sanitizeName(inst.CallTarget)
		if strings.HasPrefix(inst.CallTarget, "llvm.") {
			// LLVM intrinsic
			c.emitIntrinsic(inst, fn)
			return
		}
		var argParts []string
		calledFn := c.findFunction(inst.CallTarget)
		for i, arg := range inst.Args {
			name := paramName(i, inst.CallTarget)
			if calledFn != nil && i < len(calledFn.Params) {
				name = paramName(i, inst.CallTarget)
			}
			argParts = append(argParts, fmt.Sprintf("%s = %s", name, c.val(arg)))
		}
		argStr := strings.Join(argParts, " ")
		if inst.Result != "" && inst.Type.Kind != VoidType {
			c.line("%s = %s( %s ).", dst, target, argStr)
		} else {
			c.line("%s( %s ).", target, argStr)
		}

	default:
		if inst.Op != "" {
			c.line("\" TODO: %s", inst.Op)
		}
	}
}

func (c *abapCompiler) emitIntrinsic(inst *Instruction, fn *Function) {
	dst := c.ssaName(inst.Result)
	switch {
	case strings.Contains(inst.CallTarget, "abs"):
		c.line("%s = abs( %s ).", dst, c.val(inst.Args[0]))
	case strings.Contains(inst.CallTarget, "smax"):
		a, b := c.val(inst.Args[0]), c.val(inst.Args[1])
		c.line("IF %s > %s. %s = %s. ELSE. %s = %s. ENDIF.", a, b, dst, a, dst, b)
	case strings.Contains(inst.CallTarget, "smin"):
		a, b := c.val(inst.Args[0]), c.val(inst.Args[1])
		c.line("IF %s < %s. %s = %s. ELSE. %s = %s. ENDIF.", a, b, dst, a, dst, b)
	default:
		c.line("\" TODO: intrinsic %s", inst.CallTarget)
	}
}

func (c *abapCompiler) findFunction(name string) *Function {
	for _, fn := range c.mod.Functions {
		if fn.Name == name {
			return fn
		}
	}
	return nil
}

func (c *abapCompiler) isParam(name string) bool {
	_, ok := c.paramMap[name]
	return ok
}

func (c *abapCompiler) ssaName(reg string) string {
	if n, ok := c.paramMap[reg]; ok {
		return n
	}
	// %3 → lv_3, %result → lv_result
	name := strings.TrimPrefix(reg, "%")
	return "lv_" + name
}

func (c *abapCompiler) val(v string) string {
	v = strings.TrimSpace(v)
	// Numeric literal
	if len(v) > 0 && (v[0] >= '0' && v[0] <= '9' || v[0] == '-') {
		return v
	}
	// Boolean
	if v == "true" {
		return "1"
	}
	if v == "false" {
		return "0"
	}
	// SSA register
	if strings.HasPrefix(v, "%") {
		return c.ssaName(v)
	}
	return v
}

func (c *abapCompiler) line(format string, args ...any) {
	prefix := strings.Repeat("  ", c.indent)
	c.sb.WriteString(prefix)
	c.sb.WriteString(fmt.Sprintf(format, args...))
	c.sb.WriteByte('\n')
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "$", "_")
	if len(name) > 30 {
		name = name[:30]
	}
	return name
}

func paramName(idx int, fnName string) string {
	// Use meaningful names for common patterns
	names := []string{"a", "b", "c", "d", "e", "f_", "g_", "h"}
	if idx < len(names) {
		return names[idx]
	}
	return fmt.Sprintf("p%d", idx)
}
