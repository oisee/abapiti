package wasm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const DefaultClassLines = 20000
const maxChunkMethods = 500

type CompileResult struct {
	MainClass    string
	StateClass   string
	StateName    string
	ChunkClasses map[string]string
	Interfaces   map[string]string
	// RuntimeClass is retained for API compatibility; helpers now live in StateClass.
	RuntimeClass string
	Stats        CompileStats
}
type CompileStats struct {
	TotalFunctions, DuplicateFunctions, SavedInstructions int
	ChunkCount, TotalLines, FuncsPerChunk                 int
	CrossChunkCalls, IndirectCalls, MaxClassLines         int
}

// Files rejects duplicate ABAP object names before assembling output files.
func (r *CompileResult) Files(base string) (map[string]string, error) {
	files := map[string]string{}
	objects := map[string]bool{}
	add := func(name, extension, src string) error {
		if err := checkABAPNesting(src); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		name = strings.ToLower(sanitizeABAP(name))
		if objects[name] {
			return fmt.Errorf("duplicate generated ABAP object name %q", name)
		}
		objects[name] = true
		files[name+extension] = src
		return nil
	}
	if err := add(base, ".clas.abap", r.MainClass); err != nil {
		return nil, err
	}
	if err := add(r.StateName, ".clas.abap", r.StateClass); err != nil {
		return nil, err
	}
	for name, src := range r.ChunkClasses {
		if err := add(name, ".clas.abap", src); err != nil {
			return nil, err
		}
	}
	for name, src := range r.Interfaces {
		if err := add(name, ".intf.abap", src); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// splitStem reserves suffix space and avoids collisions with the facade.
func splitStem(base string, chunks int) string {
	digits := len(fmt.Sprint(chunks))
	if digits < 2 {
		digits = 2
	}
	stem := base[:min(len(base), 28-digits)]
	for {
		collision := stem+"_st" == base
		for g := 0; g < chunks && !collision; g++ {
			collision = fmt.Sprintf("%s_c%02d", stem, g+1) == base || chunkInterface(stem, g) == base
		}
		if !collision {
			return stem
		}
		stem = stem[:len(stem)-1]
	}
}

// CompileMultiClass uses generated line counts and call graph adjacency to pack
// functions. Oversized SCCs are divided because activation limits take priority.
func CompileMultiClass(mod *Module, base string, classLines int) (output *CompileResult, err error) {
	defer catchCompileError(&err)
	for i, f := range mod.Functions {
		for _, op := range f.Code {
			if op.Op == OpCallIndirect && op.TypeIndex >= 0 && op.TypeIndex < len(mod.Types) && len(mod.Types[op.TypeIndex].Results) > 1 {
				return nil, fmt.Errorf("split mode does not support multi-result call_indirect type %d", op.TypeIndex)
			}
		}
		if f.Type != nil && len(f.Type.Results) > 1 {
			return nil, fmt.Errorf("split mode does not support multi-result function %d (%d results)", i+mod.NumImportedFuncs, len(f.Type.Results))
		}
	}
	for _, imp := range mod.Imports {
		if imp.Kind == 0 && imp.Type != nil && len(imp.Type.Results) > 1 {
			return nil, fmt.Errorf("split mode does not support multi-result import %s (%d results)", imp.Name, len(imp.Type.Results))
		}
	}
	if classLines <= 0 {
		classLines = DefaultClassLines
	}
	base = strings.ToLower(sanitizeABAP(base))
	if len(base) > 30 {
		return nil, fmt.Errorf("ABAP facade name exceeds 30 characters: %q", base)
	}
	stem := splitStem(base, 1)
	state := stem + "_st"
	used := map[string]bool{}
	names := moduleFunctionNames(mod)
	if (&compiler{mod: mod}).hasWASI() {
		used["mem_st_i64"] = true
	}
	costs := make([]int, len(mod.Functions))
	for i := range mod.Functions {
		c := &compiler{mod: mod, names: names, copyParams: true, usedRuntime: used, splitState: state, chunkAssign: make([]int, len(mod.Functions)), chunkIndex: -1, splitInterface: chunkInterface(stem, 0)}
		if mod.Functions[i].Type != nil {
			c.emitMethodSignature(c.bodyName(i), mod.Functions[i].Type, true)
			c.emitFunction(c.splitInterface+"~"+c.bodyName(i), &mod.Functions[i])
		}
		costs[i] = strings.Count(wrapLongLines(c.sb.String()), "\n")
	}
	groups := clusterFunctions(mod, costs, classLines)
	stem = splitStem(base, len(groups))
	state = stem + "_st"
	assign := make([]int, len(mod.Functions))
	for g, funcs := range groups {
		for _, i := range funcs {
			assign[i] = g
		}
	}
	r := &CompileResult{StateName: state, ChunkClasses: map[string]string{}, Interfaces: map[string]string{}, Stats: CompileStats{TotalFunctions: len(mod.Functions), ChunkCount: len(groups), FuncsPerChunk: maxChunkMethods}}
	for g, funcs := range groups {
		name := fmt.Sprintf("%s_c%02d", stem, g+1)
		c := &compiler{mod: mod, names: names, className: name, copyParams: true, usedRuntime: used, splitState: state, chunkAssign: assign, chunkIndex: g, splitInterface: chunkInterface(stem, g)}
		c.line("INTERFACE %s PUBLIC.", c.splitInterface)
		for _, i := range funcs {
			c.emitMethodSignature(c.bodyName(i), mod.Functions[i].Type, true)
		}
		c.line("ENDINTERFACE.")
		r.Interfaces[c.splitInterface] = wrapLongLines(c.sb.String())
		c.sb.Reset()
		c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", name)
		c.line("PUBLIC SECTION.")
		c.line("INTERFACES %s.", c.splitInterface)
		c.line("ENDCLASS.")
		c.line("CLASS %s IMPLEMENTATION.", name)
		for _, i := range funcs {
			c.emitFunction(c.splitInterface+"~"+c.bodyName(i), &mod.Functions[i])
		}
		c.line("ENDCLASS.")
		r.ChunkClasses[name] = wrapLongLines(c.sb.String())
		r.Stats.CrossChunkCalls += c.crossChunkCalls
		r.Stats.IndirectCalls += c.indirectCalls
	}
	// Reuse the single-class declarations and helpers with no WASM functions.
	empty := *mod
	empty.Functions = nil
	c := &compiler{mod: &empty, className: state, copyParams: true, usedRuntime: used, ownsMemory: true}
	c.emitDefinition()
	def := c.sb.String()
	c.sb.Reset()
	def = strings.ReplaceAll(def, "METHODS constructor.", "METHODS init.")
	def = strings.ReplaceAll(def, "  PRIVATE SECTION.\n", "")
	def = strings.ReplaceAll(def, "    DATA ", "    CLASS-DATA ")
	privateMemory := "  PRIVATE SECTION.\n    CLASS-DATA mv_mem TYPE xstring.\n    CLASS-DATA mv_mem_pages TYPE i.\n"
	def = strings.ReplaceAll(def, "    CLASS-DATA mv_mem TYPE xstring.\n", "")
	def = strings.ReplaceAll(def, "    CLASS-DATA mv_mem_pages TYPE i.\n", "")
	def = strings.ReplaceAll(def, "METHODS ", "CLASS-METHODS ")
	tableDef := `    CLASS-METHODS wasi_reset.
    TYPES: BEGIN OF ty_func,
             chunk TYPE i,
             fid TYPE i,
             sig TYPE i,
           END OF ty_func.
    CLASS-DATA mt_funcs TYPE STANDARD TABLE OF ty_func WITH DEFAULT KEY.
`
	for g := range groups {
		tableDef += fmt.Sprintf("    CLASS-DATA go_c%02d TYPE REF TO %s.\n", g+1, chunkInterface(stem, g))
	}

	imports := &compiler{mod: mod, usedRuntime: used}
	for i := 0; i < mod.NumImportedFuncs; i++ {
		imp := imports.findImport(i)
		imports.emitMethodSignature(fmt.Sprintf("imp%d", i), imp.Type, true)
	}
	dispatchDef, dispatchImpl := splitDispatch(mod, state, assign)
	def = strings.Replace(def, "ENDCLASS.", tableDef+strings.ReplaceAll(imports.sb.String(), "METHODS ", "CLASS-METHODS ")+dispatchDef+privateMemory+"ENDCLASS.", 1)
	c.line("CLASS %s IMPLEMENTATION.", state)
	c.emitConstructor()
	init := c.sb.String()
	c.sb.Reset()
	init = strings.Replace(init, "METHOD constructor.", "METHOD init.", 1)
	clear := "    DATA ls_func TYPE ty_func.\n    DATA lv_name TYPE string.\n    CLEAR: mv_mem, mv_mem_pages, mt_funcs.\n"

	if c.hasWASI() {
		clear += "    CLEAR: mv_stdout, mv_stderr, mv_stdin, mv_stdin_pos, mv_exited, mv_closed0, mv_closed1, mv_closed2, mv_clock_last_us, mv_clock_wrap_us, mt_args, mt_env.\n    mv_clock_override_ts = -1.\n    mv_runtime_override_us = -1.\n"
	}
	for i := range mod.Globals {
		clear += fmt.Sprintf("    CLEAR mv_g%d.\n", i)
	}
	_, tabs := elementTables(mod)
	for _, t := range tabs {
		clear += fmt.Sprintf("    CLEAR mt_tab%d.\n", t)
	}
	init = strings.Replace(init, "METHOD init.\n", "METHOD init.\n"+clear, 1)
	var entries strings.Builder
	for g := range groups {
		fmt.Fprintf(&entries, "    IF go_c%02d IS INITIAL.\n      lv_name = '%s_C%02d'.\n      CREATE OBJECT go_c%02d TYPE (lv_name).\n    ENDIF.\n", g+1, strings.ToUpper(stem), g+1, g+1)
	}
	for i := 0; i < mod.NumImportedFuncs; i++ {
		imp := imports.findImport(i)
		fmt.Fprintf(&entries, "    CLEAR ls_func.\n    ls_func-chunk = 0.\n    ls_func-fid = %d.\n    ls_func-sig = %d.\n    APPEND ls_func TO mt_funcs.\n", i, structuralSignature(mod, imp.Type))
	}
	for i, f := range mod.Functions {
		sig := -1
		for j := range mod.Types {
			if sameFuncType(f.Type, &mod.Types[j]) {
				sig = j
				break
			}
		}
		fmt.Fprintf(&entries, "    CLEAR ls_func.\n    ls_func-chunk = %d.\n    ls_func-fid = %d.\n    ls_func-sig = %d.\n    APPEND ls_func TO mt_funcs.\n", assign[i]+1, i+mod.NumImportedFuncs, sig)
	}
	init = strings.Replace(init, "ENDMETHOD.", entries.String()+"  ENDMETHOD.", 1)
	imports.sb.Reset()
	for i := 0; i < mod.NumImportedFuncs; i++ {
		imp := imports.findImport(i)
		imports.line("METHOD imp%d.", i)
		for _, decl := range []string{"lv_wptr TYPE i", "lv_wlen TYPE i", "lv_wiov TYPE i", "lv_wn TYPE i", "lv_wbytes TYPE xstring"} {
			imports.line("DATA %s.", decl)
		}
		var args []string
		for j := range imp.Type.Params {
			args = append(args, fmt.Sprintf("p%d", j))
		}
		result := ""
		if len(imp.Type.Results) == 1 {
			result = "rv"
		}
		imports.emitSplitWASI(imp, args, result)
		imports.line("ENDMETHOD.")
	}
	c.sb.WriteString(imports.sb.String())
	c.sb.WriteString(dispatchImpl)
	c.emitWASIImplementation()
	c.line("METHOD wasi_reset.")
	c.emitWASIReset()
	c.line("ENDMETHOD.")
	c.emitMemoryHelpers()
	c.emitRuntimeHelpers()
	c.line("ENDCLASS.")
	r.StateClass = wrapLongLines(def + init + c.sb.String())
	c = &compiler{mod: mod, className: base}
	c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", base)
	c.line("PUBLIC SECTION.")
	c.line("METHODS constructor.")
	if c.hasWASI() {
		c.line("METHODS wasi_output RETURNING VALUE(rv) TYPE xstring.")
		c.line("METHODS wasi_exit RETURNING VALUE(rv) TYPE i.")
	}
	if c.hasWASI() {
		c.sb.WriteString(splitWASIAccessors(state, false))
	}
	exports := splitExports(mod)
	exportNames := splitExportNames(mod, exports)
	for ei, exp := range exports {
		c.emitMethodSignature(exportNames[ei], splitExportType(mod, exp.Index), true)
	}
	c.line("ENDCLASS.")
	c.line("CLASS %s IMPLEMENTATION.", base)
	c.line("METHOD constructor.")
	c.line("%s=>init( ).", state)
	c.line("ENDMETHOD.")
	for ei, exp := range exports {
		i := exp.Index
		ft := splitExportType(mod, i)
		c.line("METHOD %s.", exportNames[ei])
		if c.hasWASI() {
			c.line("%s=>wasi_reset( ).", state)
		}
		var args []string
		for j := range ft.Params {
			args = append(args, fmt.Sprintf("p%d = p%d", j, j))
		}
		prefix := ""
		if len(ft.Results) > 0 {
			prefix = "rv = "
		}
		if i < 0 {
			c.line("%s%s=>imp%d( %s ).", prefix, state, i+mod.NumImportedFuncs, strings.Join(args, " "))
		} else {
			c.line("%s%s=>go_c%02d->%s( %s ).", prefix, state, assign[i]+1, c.bodyName(i), strings.Join(args, " "))
		}
		c.line("ENDMETHOD.")
	}
	if c.hasWASI() {
		c.line("METHOD wasi_output.")
		c.line("rv = %s=>get_stdout( ).", state)
		c.line("ENDMETHOD.")
		c.line("METHOD wasi_exit.")
		c.line("rv = %s=>get_exit_code( ).", state)
		c.line("ENDMETHOD.")
	}
	if c.hasWASI() {
		c.sb.WriteString(splitWASIAccessors(state, true))
	}
	c.line("ENDCLASS.")
	r.MainClass = wrapLongLines(c.sb.String())
	files, err := r.Files(base)
	if err != nil {
		return nil, err
	}
	for _, src := range files {
		n := strings.Count(src, "\n")
		r.Stats.TotalLines += n
		if n > r.Stats.MaxClassLines {
			r.Stats.MaxClassLines = n
		}
	}
	return r, nil
}

// Tarjan SCCs keep recursive call groups together, then greedy adjacency packs
// callers beside callees while respecting line and method budgets.
func clusterFunctions(mod *Module, costs []int, budget int) [][]int {
	n := len(costs)
	edges := make([][]int, n)
	for i, f := range mod.Functions {
		for _, op := range f.Code {
			j := op.FuncIndex - mod.NumImportedFuncs
			if op.Op == OpCall && j >= 0 && j < n {
				edges[i] = append(edges[i], j)
			}
		}
	}
	index := make([]int, n)
	low := make([]int, n)
	on := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	next := 0
	var stack []int
	var components [][]int
	var visit func(int)
	visit = func(v int) {
		index[v] = next
		low[v] = next
		next++
		stack = append(stack, v)
		on[v] = true
		for _, w := range edges[v] {
			if index[w] < 0 {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if on[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var s []int
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				s = append(s, w)
				if w == v {
					break
				}
			}
			sort.Ints(s)
			components = append(components, s)
		}
	}
	for i := range index {
		if index[i] < 0 {
			visit(i)
		}
	}
	var groups [][]int
	var sizes []int
	assigned := make([]int, n)
	for i := range assigned {
		assigned[i] = -1
	}
	for _, component := range components {
		for len(component) > 0 {
			count, total := 0, 8
			for count < len(component) && count < maxChunkMethods {
				cost := costs[component[count]]
				if count > 0 && total+cost > budget {
					break
				}
				total += cost
				count++
			}
			part := component[:count]
			component = component[count:]
			best := -1
			score := -1
			for g := range groups {
				if sizes[g]+total > budget || len(groups[g])+count > maxChunkMethods {
					continue
				}
				s := 0
				for _, v := range part {
					for _, w := range edges[v] {
						if assigned[w] == g {
							s++
						}
					}
				}
				if s > score {
					best = g
					score = s
				}
			}
			if best < 0 {
				best = len(groups)
				groups = append(groups, nil)
				sizes = append(sizes, 8)
			}
			groups[best] = append(groups[best], part...)
			sizes[best] += total - 8
			for _, v := range part {
				assigned[v] = best
			}
		}
	}
	return groups
}

var sharedTokenRE = regexp.MustCompile(`\b(?:mv_g[0-9]+|mt_tab[0-9]+)\b`)
var helperCallRE = regexp.MustCompile(`\b([a-z][a-z0-9_]*)\(`)

func (c *compiler) splitStatement(stmt string) string {
	// Runtime memory methods own the state buffer; callers pass only operands.
	stmt = strings.ReplaceAll(stmt, "iv_mem = mv_mem ", "")
	stmt = strings.ReplaceAll(stmt, " CHANGING cv_mem = mv_mem", "")
	stmt = strings.ReplaceAll(stmt, "( EXPORTING ", "( ")
	stmt = sharedTokenRE.ReplaceAllString(stmt, c.splitState+"=>$0")
	stmt = strings.ReplaceAll(stmt, c.splitState+"=>"+c.splitState+"=>", c.splitState+"=>")
	return helperCallRE.ReplaceAllStringFunc(stmt, func(call string) string {
		name := strings.TrimSuffix(call, "(")
		if strings.HasPrefix(name, "mem_") || strings.HasPrefix(name, "i32_") || c.usedRuntime[name] {
			return c.splitState + "=>" + call
		}
		return call
	})
}
func chunkInterface(stem string, chunk int) string {
	if strings.HasPrefix(stem, "zcl_") {
		return fmt.Sprintf("zif_%s_c%02d", strings.TrimPrefix(stem, "zcl_"), chunk+1)
	}
	return fmt.Sprintf("%s_i%02d", stem, chunk+1)
}

func (c *compiler) emitStaticSplitCall(target string, args []string, result string, dispatch bool) {
	var params []string
	for i, a := range args {
		name := fmt.Sprintf("p%d", i)
		if dispatch {
			name = fmt.Sprintf("p%d", i-1)
			if i == 0 {
				name = "iv_func"
			}
		}
		params = append(params, name+" = "+a)
	}
	prefix := ""
	if result != "" {
		prefix = result + " = "
	}
	c.line("%s%s( %s ).", prefix, target, strings.Join(params, " "))
}

// Emit one typed dispatcher per structural signature used by call_indirect.
func splitDispatch(mod *Module, state string, assign []int) (string, string) {
	sigs := map[int]bool{}
	for _, f := range mod.Functions {
		for _, op := range f.Code {
			if op.Op == OpCallIndirect && op.TypeIndex >= 0 && op.TypeIndex < len(mod.Types) {
				sigs[structuralSignature(mod, &mod.Types[op.TypeIndex])] = true
			}
		}
	}
	var order []int
	for sig := range sigs {
		order = append(order, sig)
	}
	sort.Ints(order)
	d := &compiler{mod: mod}
	impl := &compiler{mod: mod}
	tables, _ := elementTables(mod)
	targets := map[int]bool{}
	for _, slots := range tables {
		for _, fid := range slots {
			if fid >= 0 {
				targets[fid] = true
			}
		}
	}
	for _, sig := range order {
		ft := &mod.Types[sig]
		d.line("CLASS-METHODS dispatch_s%d IMPORTING iv_func TYPE i", sig)
		var args []string
		for i, typ := range ft.Params {
			d.line("p%d TYPE %s", i, typ.ABAPType())
			args = append(args, fmt.Sprintf("p%d", i))
		}
		result := ""
		if len(ft.Results) == 1 {
			d.line("RETURNING VALUE(rv) TYPE %s", ft.Results[0].ABAPType())
			result = "rv"
		}
		d.line(".")
		impl.line("METHOD dispatch_s%d.", sig)
		impl.line("CASE iv_func.")
		for fid := 0; fid < mod.NumImportedFuncs+len(mod.Functions); fid++ {
			if !targets[fid] || !sameFuncType(ft, splitExportType(mod, fid-mod.NumImportedFuncs)) {
				continue
			}
			impl.line("WHEN %d.", fid)
			target := fmt.Sprintf("%s=>imp%d", state, fid)
			if fid >= mod.NumImportedFuncs {
				i := fid - mod.NumImportedFuncs
				target = fmt.Sprintf("%s=>go_c%02d->%s", state, assign[i]+1, impl.bodyName(i))
			}
			impl.emitStaticSplitCall(target, args, result, false)
		}
		impl.line("WHEN OTHERS.")
		impl.line("%s", callIndirectTrap)
		impl.line("ENDCASE.")
		impl.line("ENDMETHOD.")
	}
	return d.sb.String(), impl.sb.String()
}

// Module.Exports retains aliases that Function.ExportName cannot represent.
// Index in the returned list is local to Module.Functions.
func splitExports(mod *Module) []Export {
	var exports []Export
	seen := map[string]bool{}
	for _, exp := range mod.Exports {
		i := exp.Index - mod.NumImportedFuncs
		if exp.Kind == 0 && i >= -mod.NumImportedFuncs && i < len(mod.Functions) && splitExportType(mod, i) != nil {
			exp.Index = i
			exports = append(exports, exp)
			seen[exp.Name] = true
		}
	}
	for i, f := range mod.Functions {
		if f.ExportName != "" && f.Type != nil && !seen[f.ExportName] {
			exports = append(exports, Export{Name: f.ExportName, Index: i})
		}
	}
	return exports
}

func splitExportType(mod *Module, localIndex int) *FuncType {
	if localIndex >= 0 {
		return mod.Functions[localIndex].Type
	}
	c := &compiler{mod: mod}
	return c.findImport(localIndex + mod.NumImportedFuncs).Type
}
func structuralSignature(mod *Module, ft *FuncType) int {
	for i := range mod.Types {
		if sameFuncType(ft, &mod.Types[i]) {
			return i
		}
	}
	return -1
}

// Allocate aliases and imported exports in the same namespace as function names.
func splitExportNames(mod *Module, exports []Export) []string {
	c := &compiler{mod: mod}
	var a abapNameAllocator
	for name := range internalNames {
		a.reserve(name)
	}
	declarations, _ := runtimeTemplates()
	for name := range declarations {
		a.reserve(name)
	}
	for i := range mod.Functions {
		a.reserve(c.functionName(i))
		a.reserve(c.bodyName(i))
	}
	names := make([]string, len(exports))
	for ei, exp := range exports {
		if exp.Index >= 0 && exp.Name == mod.Functions[exp.Index].ExportName {
			names[ei] = c.functionName(exp.Index)
			continue
		}
		raw := sanitizeABAP(exp.Name)
		if looksInternal(raw) {
			raw = "e_" + raw
		}
		names[ei] = a.allocate(raw)
	}
	return names
}

func splitWASIAccessors(state string, implementation bool) string {
	var b strings.Builder
	for _, accessor := range []struct{ name, param, typ string }{
		{"get_stdout", "", "xstring"}, {"get_stderr", "", "xstring"}, {"get_exit_code", "", "i"},
		{"set_stdin", "iv", "xstring"}, {"set_args", "it", "string_table"}, {"set_env", "it", "string_table"},
	} {
		if implementation {
			fmt.Fprintf(&b, "METHOD %s.\n", accessor.name)
			if accessor.param == "" {
				fmt.Fprintf(&b, "rv = %s=>%s( ).\n", state, accessor.name)
			} else {
				fmt.Fprintf(&b, "%s=>%s( %s = %s ).\n", state, accessor.name, accessor.param, accessor.param)
			}
			b.WriteString("ENDMETHOD.\n")
		} else if accessor.param == "" {
			fmt.Fprintf(&b, "METHODS %s RETURNING VALUE(rv) TYPE %s.\n", accessor.name, accessor.typ)
		} else {
			fmt.Fprintf(&b, "METHODS %s IMPORTING %s TYPE %s.\n", accessor.name, accessor.param, accessor.typ)
		}
	}
	return b.String()
}
