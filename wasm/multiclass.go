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
	// RuntimeClass is retained for API compatibility; helpers now live in StateClass.
	RuntimeClass string
	Stats        CompileStats
}
type CompileStats struct {
	TotalFunctions, DuplicateFunctions, SavedInstructions int
	ChunkCount, TotalLines, FuncsPerChunk                 int
	CrossChunkCalls, IndirectCalls, MaxClassLines         int
}

func (r *CompileResult) Files(base string) map[string]string {
	files := map[string]string{strings.ToLower(base) + ".clas.abap": r.MainClass, r.StateName + ".clas.abap": r.StateClass}
	for name, src := range r.ChunkClasses {
		files[name+".clas.abap"] = src
	}
	return files
}

// CompileMultiClass uses generated line counts and call graph adjacency to pack
// functions. Oversized SCCs are divided because activation limits take priority.
func CompileMultiClass(mod *Module, base string, classLines int) *CompileResult {
	if classLines <= 0 {
		classLines = DefaultClassLines
	}
	base = strings.ToLower(sanitizeABAP(base))
	stem := base
	if len(stem) > 26 {
		stem = stem[:26]
	}
	state := stem + "_st"
	used := map[string]bool{}
	costs := make([]int, len(mod.Functions))
	for i := range mod.Functions {
		c := &compiler{mod: mod, copyParams: true, usedRuntime: used, splitState: state, chunkAssign: make([]int, len(mod.Functions)), chunkIndex: -1}
		if mod.Functions[i].Type != nil {
			c.emitMethodSignature(fmt.Sprintf("f%d", i), mod.Functions[i].Type, true)
			c.emitFunction(fmt.Sprintf("f%d", i), &mod.Functions[i])
		}
		costs[i] = strings.Count(wrapLongLines(c.sb.String()), "\n")
	}
	groups := clusterFunctions(mod, costs, classLines)
	// Reserve additional suffix digits only when there are over 99 chunks.
	if digits := len(fmt.Sprint(len(groups))); digits > 2 && len(stem) > 28-digits {
		stem = stem[:28-digits]
		state = stem + "_st"
	}
	assign := make([]int, len(mod.Functions))
	for g, funcs := range groups {
		for _, i := range funcs {
			assign[i] = g
		}
	}
	r := &CompileResult{StateName: state, ChunkClasses: map[string]string{}, Stats: CompileStats{TotalFunctions: len(mod.Functions), ChunkCount: len(groups), FuncsPerChunk: maxChunkMethods}}
	for g, funcs := range groups {
		name := fmt.Sprintf("%s_c%02d", stem, g+1)
		c := &compiler{mod: mod, className: name, copyParams: true, usedRuntime: used, splitState: state, chunkAssign: assign, chunkIndex: g}
		c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", name)
		c.line("PUBLIC SECTION.")
		for _, i := range funcs {
			c.emitMethodSignature(fmt.Sprintf("f%d", i), mod.Functions[i].Type, true)
		}
		c.line("ENDCLASS.")
		c.line("CLASS %s IMPLEMENTATION.", name)
		for _, i := range funcs {
			c.emitFunction(fmt.Sprintf("f%d", i), &mod.Functions[i])
		}
		c.line("ENDCLASS.")
		r.ChunkClasses[name] = wrapLongLines(c.sb.String())
		r.Stats.CrossChunkCalls += c.crossChunkCalls
		r.Stats.IndirectCalls += c.indirectCalls
	}
	// Reuse the single-class declarations and helpers with no WASM functions.
	empty := *mod
	empty.Functions = nil
	c := &compiler{mod: &empty, className: state, copyParams: true, usedRuntime: used}
	c.emitDefinition()
	def := c.sb.String()
	c.sb.Reset()
	def = strings.ReplaceAll(def, "METHODS constructor.", "METHODS init.")
	def = strings.ReplaceAll(def, "  PRIVATE SECTION.\n", "")
	def = strings.ReplaceAll(def, "    DATA ", "    CLASS-DATA ")
	def = strings.ReplaceAll(def, "METHODS ", "CLASS-METHODS ")
	tableDef := `    TYPES: BEGIN OF ty_func,
             cls TYPE string,
             meth TYPE string,
             sig TYPE i,
           END OF ty_func.
    CLASS-DATA mt_funcs TYPE STANDARD TABLE OF ty_func WITH DEFAULT KEY.
`
	if mod.NumImportedFuncs > 0 {
		tableDef += "    CLASS-DATA mv_wasi_output TYPE xstring.\n    CLASS-DATA mv_wasi_exit TYPE i.\n"
	}
	def = strings.Replace(def, "ENDCLASS.", tableDef+"ENDCLASS.", 1)
	c.line("CLASS %s IMPLEMENTATION.", state)
	c.emitConstructor()
	init := c.sb.String()
	c.sb.Reset()
	init = strings.Replace(init, "METHOD constructor.", "METHOD init.", 1)
	clear := "    DATA ls_func TYPE ty_func.\n    CLEAR: mv_mem, mv_mem_pages, mt_funcs.\n"
	if mod.NumImportedFuncs > 0 {
		clear += "    CLEAR: mv_wasi_output, mv_wasi_exit.\n"
	}
	for i := range mod.Globals {
		clear += fmt.Sprintf("    CLEAR mv_g%d.\n", i)
	}
	_, tabs := elementTables(mod)
	for _, t := range tabs {
		clear += fmt.Sprintf("    CLEAR mt_tab%d.\n", t)
	}
	init = strings.Replace(init, "METHOD init.\n", "METHOD init.\n"+clear, 1)
	for i := 0; i < mod.NumImportedFuncs; i++ {
		init = strings.Replace(init, "ENDMETHOD.", "    CLEAR ls_func.\n    ls_func-sig = -1.\n    APPEND ls_func TO mt_funcs.\n  ENDMETHOD.", 1)
	}
	var entries strings.Builder
	for i, f := range mod.Functions {
		sig := -1
		for j := range mod.Types {
			if sameFuncType(f.Type, &mod.Types[j]) {
				sig = j
				break
			}
		}
		fmt.Fprintf(&entries, "    CLEAR ls_func.\n    CONCATENATE '%s' '_C%02d' INTO ls_func-cls.\n    ls_func-meth = 'F%d'.\n    ls_func-sig = %d.\n    APPEND ls_func TO mt_funcs.\n", strings.ToUpper(stem), assign[i]+1, i, sig)
	}
	init = strings.Replace(init, "ENDMETHOD.", entries.String()+"  ENDMETHOD.", 1)
	c.emitMemoryHelpers()
	c.emitRuntimeHelpers()
	c.line("ENDCLASS.")
	r.StateClass = wrapLongLines(def + init + c.sb.String())
	c = &compiler{mod: mod, className: base}
	c.line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", base)
	c.line("PUBLIC SECTION.")
	c.line("METHODS constructor.")
	if mod.NumImportedFuncs > 0 {
		c.line("METHODS wasi_output RETURNING VALUE(rv) TYPE xstring.")
		c.line("METHODS wasi_exit RETURNING VALUE(rv) TYPE i.")
	}
	exports := splitExports(mod)
	for _, exp := range exports {
		c.emitMethodSignature(exp.Name, mod.Functions[exp.Index].Type, true)
	}
	c.line("ENDCLASS.")
	c.line("CLASS %s IMPLEMENTATION.", base)
	c.line("METHOD constructor.")
	c.line("%s=>init( ).", state)
	c.line("ENDMETHOD.")
	for _, exp := range exports {
		i := exp.Index
		f := mod.Functions[i]
		c.line("METHOD %s.", sanitizeABAP(exp.Name))
		var args []string
		for j := range f.Type.Params {
			args = append(args, fmt.Sprintf("p%d = p%d", j, j))
		}
		prefix := ""
		if len(f.Type.Results) > 0 {
			prefix = "rv = "
		}
		c.line("%s%s_c%02d=>f%d( %s ).", prefix, stem, assign[i]+1, i, strings.Join(args, " "))
		c.line("ENDMETHOD.")
	}
	if mod.NumImportedFuncs > 0 {
		c.line("METHOD wasi_output.")
		c.line("rv = %s=>mv_wasi_output.", state)
		c.line("ENDMETHOD.")
		c.line("METHOD wasi_exit.")
		c.line("rv = %s=>mv_wasi_exit.", state)
		c.line("ENDMETHOD.")
	}
	c.line("ENDCLASS.")
	r.MainClass = wrapLongLines(c.sb.String())
	for _, src := range r.Files(base) {
		n := strings.Count(src, "\n")
		r.Stats.TotalLines += n
		if n > r.Stats.MaxClassLines {
			r.Stats.MaxClassLines = n
		}
	}
	return r
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

var sharedTokenRE = regexp.MustCompile(`\b(?:mv_mem_pages|mv_mem|mv_g[0-9]+|mt_tab[0-9]+)\b`)
var helperCallRE = regexp.MustCompile(`\b([a-z][a-z0-9_]*)\(`)

func (c *compiler) splitStatement(stmt string) string {
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
func (c *compiler) emitDynamicCall(args []string, result string) {
	c.line("lv_cls = ls_ci-cls.")
	c.line("lv_meth = ls_ci-meth.")
	s := "CALL METHOD (lv_cls)=>(lv_meth)"
	if len(args) > 0 {
		s += " EXPORTING"
		for i, a := range args {
			s += fmt.Sprintf(" p%d = %s", i, a)
		}
	}
	if result != "" {
		s += " RECEIVING rv = " + result
	}
	c.line("%s.", s)
}

// Module.Exports retains aliases that Function.ExportName cannot represent.
// Index in the returned list is local to Module.Functions.
func splitExports(mod *Module) []Export {
	var exports []Export
	seen := map[string]bool{}
	for _, exp := range mod.Exports {
		i := exp.Index - mod.NumImportedFuncs
		if exp.Kind == 0 && i >= 0 && i < len(mod.Functions) && mod.Functions[i].Type != nil {
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
