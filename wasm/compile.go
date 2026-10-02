package wasm

import (
	"fmt"
	"strings"
)

// CompileResult holds the output of multi-class compilation.
type CompileResult struct {
	MainClass    string            // main class source (memory, globals, exports, WASI)
	ChunkClasses map[string]string // chunk class name → source
	RuntimeClass string            // zcl_wasm_rt source
	Stats        CompileStats
}

// CompileStats contains compilation statistics.
type CompileStats struct {
	TotalFunctions     int
	DuplicateFunctions int
	SavedInstructions  int
	ChunkCount         int
	TotalLines         int
	FuncsPerChunk      int
}

// CompileMultiClass compiles a WASM module into multiple ABAP classes.
// Functions are distributed across chunk classes (max funcsPerChunk each).
// A main class holds memory, globals, exports, WASI shim, and delegates.
func CompileMultiClass(mod *Module, baseName string, funcsPerChunk int) (output *CompileResult, err error) {
	defer catchCompileError(&err)
	if funcsPerChunk <= 0 {
		funcsPerChunk = 80
	}

	// Deduplicate functions
	redirects := DeduplicateFunctions(mod)
	dupes, _, savedInstrs := DedupStats(mod, redirects)

	// Build function name map and chunk assignments
	type funcInfo struct {
		localIdx   int
		name       string
		chunkIdx   int
		isDupe     bool
		canonName  string
		canonChunk int
	}

	funcMap := make([]funcInfo, len(mod.Functions))
	activeFuncCount := 0

	for i, f := range mod.Functions {
		fi := funcInfo{localIdx: i}
		if f.ExportName != "" {
			fi.name = sanitizeABAP(f.ExportName)
		} else {
			fi.name = fmt.Sprintf("f%d", i)
		}

		if canonIdx, ok := redirects[i]; ok {
			fi.isDupe = true
			cf := mod.Functions[canonIdx]
			if cf.ExportName != "" {
				fi.canonName = sanitizeABAP(cf.ExportName)
			} else {
				fi.canonName = fmt.Sprintf("f%d", canonIdx)
			}
			fi.canonChunk = canonIdx / funcsPerChunk
			fi.chunkIdx = canonIdx / funcsPerChunk // map to canonical's chunk
		} else {
			fi.chunkIdx = activeFuncCount / funcsPerChunk
			activeFuncCount++
		}

		funcMap[i] = fi
	}

	// Determine chunk assignments for non-duplicate functions
	chunkAssign := make([]int, len(mod.Functions))
	slot := 0
	for i := range mod.Functions {
		if _, ok := redirects[i]; ok {
			// Duplicate — assigned to canonical's chunk
			canonIdx := redirects[i]
			chunkAssign[i] = chunkAssign[canonIdx]
		} else {
			chunkAssign[i] = slot / funcsPerChunk
			slot++
		}
	}

	numChunks := 0
	if slot > 0 {
		numChunks = (slot-1)/funcsPerChunk + 1
	}

	result := &CompileResult{
		ChunkClasses: make(map[string]string),
		Stats: CompileStats{
			TotalFunctions:     len(mod.Functions),
			DuplicateFunctions: dupes,
			SavedInstructions:  savedInstrs,
			ChunkCount:         numChunks,
			FuncsPerChunk:      funcsPerChunk,
		},
	}

	// Generate chunk classes
	for chunkIdx := 0; chunkIdx < numChunks; chunkIdx++ {
		chunkName := fmt.Sprintf("%s_c%02d", baseName, chunkIdx)
		src := wrapLongLines(emitChunkClass(mod, chunkName, baseName, chunkIdx, funcsPerChunk, chunkAssign, redirects))
		result.ChunkClasses[chunkName] = src
		result.Stats.TotalLines += strings.Count(src, "\n")
	}

	// Generate main class
	result.MainClass = wrapLongLines(emitMainClass(mod, baseName, numChunks, funcsPerChunk, chunkAssign, redirects))
	result.Stats.TotalLines += strings.Count(result.MainClass, "\n")

	// Generate runtime class
	result.RuntimeClass = wrapLongLines(stripABAPComments(emitRuntimeClass()))
	result.MainClass = stripABAPComments(result.MainClass)
	for name, src := range result.ChunkClasses {
		result.ChunkClasses[name] = stripABAPComments(src)
	}
	result.Stats.TotalLines += strings.Count(result.RuntimeClass, "\n")

	return result, nil
}

// --- Chunk Class ---

func emitChunkClass(mod *Module, chunkName, baseName string, chunkIdx, funcsPerChunk int, chunkAssign []int, redirects map[int]int) string {
	c := &compiler{mod: mod, className: chunkName, sharedMain: true}

	c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", chunkName)
	c.indent++
	c.line("PUBLIC SECTION.")
	c.indent++

	// Reference to main class (for memory, globals)
	c.line("DATA mo_main TYPE REF TO %s.", baseName)

	// Declare methods for functions in this chunk
	for i, f := range mod.Functions {
		if chunkAssign[i] != chunkIdx {
			continue
		}
		if _, ok := redirects[i]; ok {
			continue // skip duplicates
		}
		if f.Type == nil {
			continue
		}
		name := fmt.Sprintf("f%d", i)
		if f.ExportName != "" {
			name = sanitizeABAP(f.ExportName)
		}
		c.emitMethodSignature(name, f.Type, false)
	}

	c.indent--
	c.indent--
	c.line("ENDCLASS.")
	c.line("")
	c.line("CLASS %s IMPLEMENTATION.", chunkName)
	c.indent++

	// Emit function implementations
	for i := range mod.Functions {
		f := &mod.Functions[i]
		if chunkAssign[i] != chunkIdx {
			continue
		}
		if _, ok := redirects[i]; ok {
			continue
		}
		if f.Type == nil {
			continue
		}
		name := fmt.Sprintf("f%d", i)
		if f.ExportName != "" {
			name = sanitizeABAP(f.ExportName)
		}
		c.emitFunctionWithMainRef(name, f, baseName, chunkAssign, redirects)
	}

	c.indent--
	c.line("ENDCLASS.")

	return c.sb.String()
}

// emitFunctionWithMainRef generates a method that accesses memory/globals via mo_main.
func (c *compiler) emitFunctionWithMainRef(name string, f *Function, baseName string, chunkAssign []int, redirects map[int]int) {
	// Reuse main's typed stacks, parameter copies and inline arithmetic.
	c.copyParams = true
	c.emitFunction(name, f)
}

// --- Main Class ---

func emitMainClass(mod *Module, baseName string, numChunks, funcsPerChunk int, chunkAssign []int, redirects map[int]int) string {
	c := &compiler{mod: mod, className: baseName, usedRuntime: make(map[string]bool)}

	c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", baseName)
	c.indent++
	c.line("PUBLIC SECTION.")
	c.indent++

	c.line("METHODS constructor.")
	c.emitWASIDeclarations()
	if c.hasWASI() {
		c.usedRuntime["mem_st_i64"] = true
		c.emitRuntimeDeclarations()
	}

	// Memory and globals are public so chunk classes can access them
	c.line("DATA mv_mem TYPE xstring.")
	c.line("DATA mv_mem_pages TYPE i.")
	for i, g := range mod.Globals {
		c.line("DATA mv_g%d TYPE %s.", i, g.Type.ABAPType())
	}

	// Exported functions
	for _, f := range mod.Functions {
		if f.ExportName != "" && f.Type != nil {
			c.emitMethodSignature(f.ExportName, f.Type, true)
		}
	}

	// Memory helpers
	c.line("METHODS mem_ld_i32 IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_st_i32 IMPORTING iv_addr TYPE i iv_val TYPE i.")
	c.line("METHODS mem_ld_i32_8u IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_ld_i32_8s IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_ld_i32_16u IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_st_i32_8 IMPORTING iv_addr TYPE i iv_val TYPE i.")
	c.line("METHODS mem_st_i32_16 IMPORTING iv_addr TYPE i iv_val TYPE i.")
	c.line("METHODS mem_grow IMPORTING iv_pages TYPE i RETURNING VALUE(rv) TYPE i.")
	c.line("METHODS mem_zero_pages IMPORTING iv_pages TYPE i RETURNING VALUE(rv_mem) TYPE xstring.")

	c.indent--
	c.line("PRIVATE SECTION.")
	c.indent++

	// Chunk class references
	for i := 0; i < numChunks; i++ {
		c.line("DATA mo_c%02d TYPE REF TO %s_c%02d.", i, baseName, i)
	}

	// Function table
	_, tableIndices := elementTables(mod)
	for _, t := range tableIndices {
		c.line("DATA mt_tab%d TYPE STANDARD TABLE OF i WITH DEFAULT KEY.", t)
	}

	c.indent--
	c.indent--
	c.line("ENDCLASS.")
	c.line("")
	c.line("CLASS %s IMPLEMENTATION.", baseName)
	c.indent++

	// Constructor
	c.line("METHOD constructor.")
	c.indent++

	// Create chunk classes
	for i := 0; i < numChunks; i++ {
		c.line("CREATE OBJECT mo_c%02d.", i)
		c.line("mo_c%02d->mo_main = me.", i)
	}

	// WASI
	c.emitWASIInit()

	// Memory init
	if mod.Memory != nil {
		pages := mod.Memory.Min
		c.line("mv_mem_pages = %d.", pages)
		c.line("mv_mem = mem_zero_pages( %d ).", pages)
	}

	// Globals
	for i, g := range mod.Globals {
		if g.InitI32 != 0 {
			c.line("mv_g%d = %d.", i, g.InitI32)
		} else if g.InitI64 != 0 {
			c.line("mv_g%d = %d.", i, g.InitI64)
		}
	}

	// Data segments (REPLACE SECTION, chunked; see memhelpers.go)
	c.emitDataSegments("mv_mem")

	// Element segments
	tables, tableIndices := elementTables(mod)
	for _, t := range tableIndices {
		for _, funcIdx := range tables[t] {
			c.line("APPEND %d TO mt_tab%d.", funcIdx, t)
		}
	}

	c.indent--
	c.line("ENDMETHOD.")

	// Memory helpers
	c.emitWASIImplementation()
	c.emitRuntimeHelpers()
	c.emitMemoryHelpers()

	// Exported functions — delegate to chunk classes
	for i, f := range mod.Functions {
		if f.ExportName == "" || f.Type == nil {
			continue
		}
		c.line("METHOD %s.", sanitizeABAP(f.ExportName))
		c.indent++

		c.emitWASIReset()

		// Find the actual target (might be deduplicated)
		targetIdx := i
		if canonIdx, ok := redirects[i]; ok {
			targetIdx = canonIdx
		}
		chunkIdx := chunkAssign[targetIdx]
		targetName := fmt.Sprintf("f%d", targetIdx)
		if mod.Functions[targetIdx].ExportName != "" {
			targetName = sanitizeABAP(mod.Functions[targetIdx].ExportName)
		}

		// Build delegation call
		var params []string
		for j := range f.Type.Params {
			params = append(params, fmt.Sprintf("p%d = p%d", j, j))
		}
		paramStr := strings.Join(params, " ")

		if len(f.Type.Results) > 0 {
			if len(params) > 0 {
				c.line("rv = mo_c%02d->%s( %s ).", chunkIdx, targetName, paramStr)
			} else {
				c.line("rv = mo_c%02d->%s( ).", chunkIdx, targetName)
			}
		} else {
			if len(params) > 0 {
				c.line("mo_c%02d->%s( %s ).", chunkIdx, targetName, paramStr)
			} else {
				c.line("mo_c%02d->%s( ).", chunkIdx, targetName)
			}
		}

		c.indent--
		c.line("ENDMETHOD.")
	}

	c.indent--
	c.line("ENDCLASS.")

	return c.sb.String()
}

// --- Runtime Class ---

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
