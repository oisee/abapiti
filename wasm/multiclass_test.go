package wasm

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestSplitIndependentClasses(t *testing.T) {
	fixtures := map[string][]byte{"indirect": buildCallIndirectModule(), "i64": buildI64WrapModule(i64WrapCases), "memory": buildMemoryModule(1, true)}
	for _, name := range []string{"add", "factorial", "quickjs_eval"} {
		bin, err := os.ReadFile("testdata/" + name + ".wasm")
		if err != nil {
			t.Fatal(err)
		}
		fixtures[name] = bin
	}
	staticRef := regexp.MustCompile(`\b([a-z][a-z0-9_]*)=>`)
	for name, bin := range fixtures {
		t.Run(name, func(t *testing.T) {
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			budget := 200
			if name == "quickjs_eval" {
				budget = DefaultClassLines
			}
			r := mustCompileMultiClass(t, mod, "zcl_split_test", budget)
			seen := map[string]bool{}
			stateMethods := map[string]bool{}
			for _, m := range classMethodImplementation.FindAllStringSubmatch(r.StateClass, -1) {
				stateMethods[m[1]] = true
			}
			for _, src := range r.ChunkClasses {
				refs := regexp.MustCompile(regexp.QuoteMeta(r.StateName) + `=>([a-z][a-z0-9_]*)\(`)
				for _, ref := range refs.FindAllStringSubmatch(src, -1) {
					if !stateMethods[ref[1]] {
						t.Errorf("undefined state helper %s", ref[1])
					}
				}
			}
			if n := strings.Count(r.StateClass, "APPEND ls_func TO mt_funcs."); n != len(mod.Functions)+mod.NumImportedFuncs {
				t.Errorf("function registry has %d rows", n)
			}

			for cls, src := range r.ChunkClasses {
				for _, ref := range staticRef.FindAllStringSubmatch(src, -1) {
					if ref[1] != r.StateName {
						t.Errorf("%s depends on %s", cls, ref[1])
					}
				}
				for other := range r.ChunkClasses {
					if other != cls && strings.Contains(strings.ToLower(src), other) {
						t.Errorf("%s mentions %s", cls, other)
					}
				}
				for _, m := range regexp.MustCompile(`METHOD [a-z0-9_]+~(f[0-9]+)\.`).FindAllStringSubmatch(src, -1) {
					seen[m[1]] = true
				}
				if lines := strings.Count(src, "\n"); lines > budget {
					t.Errorf("%s: %d lines exceeds %d", cls, lines, budget)
				}
			}
			for cls := range r.ChunkClasses {
				literal := "'" + strings.ToUpper(cls) + "'"
				if !strings.Contains(r.StateClass, "lv_name = "+literal+".") {
					t.Errorf("missing uppercase creation name %s", cls)
				}
				withoutNames := strings.ReplaceAll(r.StateClass, literal, "''")
				if strings.Contains(strings.ToLower(withoutNames), cls) {
					t.Errorf("state mentions %s", cls)
				}
			}
			for iface, src := range r.Interfaces {
				if len(iface) > 30 || !strings.HasPrefix(iface, "zif_") {
					t.Errorf("invalid interface name %s", iface)
				}
				if strings.Contains(src, "CLASS-METHODS") || strings.Contains(src, r.StateName) {
					t.Errorf("interface %s has a static dependency", iface)
				}
				for _, method := range regexp.MustCompile(`METHODS (f[0-9]+)`).FindAllStringSubmatch(src, -1) {
					found := false
					for _, chunk := range r.ChunkClasses {
						found = found || strings.Contains(chunk, "METHOD "+iface+"~"+method[1]+".")
					}
					if !found {
						t.Errorf("missing implementation %s~%s", iface, method[1])
					}
				}
			}
			for _, exp := range splitExports(mod) {
				if !strings.Contains(r.MainClass, "METHOD "+sanitizeABAP(exp.Name)+".") {
					t.Errorf("export %s unreachable", exp.Name)
				}
			}
			for i, f := range mod.Functions {
				if f.Type != nil && !seen[fmt.Sprintf("f%d", i)] {
					t.Errorf("function %d missing", i)
				}
				if f.ExportName != "" && !strings.Contains(r.MainClass, "METHOD "+sanitizeABAP(f.ExportName)+".") {
					t.Errorf("export %s unreachable", f.ExportName)
				}
			}
			for file, src := range r.Files("zcl_split_test") {
				for _, bad := range []string{"DATA(", "NEW ", "|", "CONV ", "VALUE #", "zcl_wasm_rt", "PERFORM ", "CALL METHOD ("} {
					if strings.Contains(src, bad) {
						t.Errorf("%s contains %s", file, bad)
					}
				}
				for _, line := range strings.Split(src, "\n") {
					if len(line) > 255 {
						t.Errorf("%s line has %d bytes", file, len(line))
					}
				}
				assertNoABAPComments(t, file, src)
				if parameterWrite.MatchString(src) {
					t.Errorf("%s writes importing parameter", file)
				}
			}
		})
	}
}

func TestSplitCallGraphClustering(t *testing.T) {
	mod := &Module{Functions: make([]Function, 4)}
	mod.Functions[0].Code = []Instruction{{Op: OpCall, FuncIndex: 3}}
	mod.Functions[3].Code = []Instruction{{Op: OpCall, FuncIndex: 0}}
	groups := clusterFunctions(mod, []int{20, 20, 20, 20}, 50)
	for _, g := range groups {
		if len(g) == 2 && g[0] == 0 && g[1] == 3 {
			return
		}
	}
	t.Fatal("recursive functions were separated despite fitting the budget")
}

func TestSplitExportAliases(t *testing.T) {
	bin, err := os.ReadFile("testdata/add.wasm")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := Parse(bin)
	if err != nil {
		t.Fatal(err)
	}
	mod.Exports = append(mod.Exports, Export{Name: "alias_add", Kind: 0, Index: 0})
	r := mustCompileMultiClass(t, mod, "zcl_alias", 200)
	if !strings.Contains(r.MainClass, "METHOD alias_add.") || !strings.Contains(r.MainClass, "zcl_alias_st=>go_c01->f0(") {
		t.Fatal("export alias missing")
	}
}

func buildDirectSplitModule() []byte {
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Params: []ValType{ValI32}, Results: []ValType{ValI32}}}))
	w.addSection(3, buildFuncSection([]int{0, 0, 0}))
	w.addSection(7, buildExportSection([]Export{{Name: "call", Kind: 0, Index: 1}}))
	w.addSection(10, buildCodeSection([][]byte{
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpI32Const, 1, OpI32Add, OpLocalSet, 0, OpLocalGet, 0}),
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpCall, 0, OpCall, 2}),
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpI32Const, 2, OpI32Mul}),
	}))
	return w.bytes()
}

func TestSplitClassNames(t *testing.T) {
	mod, err := Parse(buildDirectSplitModule())
	if err != nil {
		t.Fatal(err)
	}
	base := "zcl_abcdefghijklmnopqrstuv"
	r := mustCompileMultiClass(t, mod, base, 40)
	if r.StateName != base+"_st" {
		t.Fatalf("state name %s", r.StateName)
	}
	for name := range r.ChunkClasses {
		if len(name) > 30 {
			t.Fatalf("class name too long: %s", name)
		}
	}
}

func mustCompileMultiClass(t *testing.T, mod *Module, base string, budget int) *CompileResult {
	t.Helper()
	r, err := CompileMultiClass(mod, base, budget)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func buildSplitImportModule() []byte {
	w := newWasmBuilder()
	ft := FuncType{Params: []ValType{ValI32, ValI32}, Results: []ValType{ValI32}}
	w.addSection(1, buildTypeSection([]FuncType{ft, ft, {Params: []ValType{ValI32, ValI32, ValI32}, Results: []ValType{ValI32}}}))
	imports := []byte{1}
	for _, name := range []string{"wasi_snapshot_preview1", "args_sizes_get"} {
		imports = append(imports, leb128u(uint32(len(name)))...)
		imports = append(imports, []byte(name)...)
	}
	imports = append(imports, 0, 1)
	w.addSection(2, imports)
	w.addSection(3, buildFuncSection([]int{2, 0}))
	w.addSection(4, []byte{1, 0x70, 0, 2})
	w.addSection(5, []byte{1, 0, 1})
	w.addSection(7, buildExportSection([]Export{
		{Name: "sizes", Kind: 0, Index: 0},
		{Name: "call", Kind: 0, Index: 1},
		{Name: "direct", Kind: 0, Index: 2},
	}))
	w.addSection(9, []byte{1, 0, OpI32Const, 1, OpEnd, 1, 0})
	w.addSection(10, buildCodeSection([][]byte{
		buildFuncBody(nil, []byte{OpLocalGet, 1, OpLocalGet, 2, OpLocalGet, 0, OpCallIndirect, 0, 0}),
		buildFuncBody(nil, []byte{OpLocalGet, 0, OpLocalGet, 1, OpCall, 0}),
	}))
	return w.bytes()
}

func TestSplitImportRegistryAndExports(t *testing.T) {
	mod, err := Parse(buildSplitImportModule())
	if err != nil {
		t.Fatal(err)
	}
	r := mustCompileMultiClass(t, mod, "zcl_import", 40)
	for _, want := range []string{
		"CLASS-METHODS imp0 IMPORTING p0 TYPE i p1 TYPE i RETURNING VALUE(rv) TYPE i.",
		"METHOD imp0.", "ls_func-chunk = 0.", "ls_func-fid = 0.", "ls_func-sig = 0.",
		"CASE iv_func.", "WHEN 0.", "rv = zcl_import_st=>imp0( p0 = p0 p1 = p1 ).",
	} {
		if !strings.Contains(r.StateClass, want) {
			t.Errorf("state missing %s", want)
		}
	}
	if !strings.Contains(r.MainClass, "METHOD sizes.") || !strings.Contains(r.MainClass, "rv = zcl_import_st=>imp0( p0 = p0 p1 = p1 ).") {
		t.Fatal("re-exported import missing")
	}
	if r.Stats.IndirectCalls != 1 {
		t.Fatal("missing indirect import call")
	}
}

func TestSplitRejectsMultiResults(t *testing.T) {
	ft := FuncType{Results: []ValType{ValI32, ValI64}}
	for _, mod := range []*Module{
		{Functions: []Function{{Type: &ft}}},
		{Types: []FuncType{ft}, Functions: []Function{{Type: &FuncType{}, Code: []Instruction{{Op: OpCallIndirect, TypeIndex: 0}}}}},
		{NumImportedFuncs: 1, Imports: []Import{{Kind: 0, Name: "multi", Type: &ft}}},
	} {
		r, err := CompileMultiClass(mod, "zcl_multi", 40)
		if r != nil || err == nil || !strings.Contains(err.Error(), "split mode does not support multi-result") {
			t.Fatalf("got %v, %v", r, err)
		}
	}
}

func TestSplitTypedInterfaceCalls(t *testing.T) {
	for _, result := range []ValType{ValI64, ValF64, 0} {
		ft := FuncType{Params: []ValType{ValI32, ValI64, ValF32, ValF64}}
		if result != 0 {
			ft.Results = []ValType{result}
		}
		w := newWasmBuilder()
		w.addSection(1, buildTypeSection([]FuncType{ft}))
		w.addSection(3, buildFuncSection([]int{0, 0, 0}))
		w.addSection(4, []byte{1, 0x70, 0, 1})
		w.addSection(7, buildExportSection([]Export{{Name: "mixed", Kind: 0, Index: 2}}))
		w.addSection(9, []byte{1, 0, OpI32Const, 0, OpEnd, 1, 0})
		var leaf []byte
		if result == ValI64 {
			leaf = []byte{OpLocalGet, 1}
		}
		if result == ValF64 {
			leaf = []byte{OpLocalGet, 3}
		}
		params := []byte{OpLocalGet, 0, OpLocalGet, 1, OpLocalGet, 2, OpLocalGet, 3}
		direct := append(append([]byte{}, params...), OpCall, 0)
		indirect := append(append([]byte{}, params...), OpI32Const, 0, OpCallIndirect, 0, 0)
		w.addSection(10, buildCodeSection([][]byte{buildFuncBody(nil, leaf), buildFuncBody(nil, direct), buildFuncBody(nil, indirect)}))
		mod, err := Parse(w.bytes())
		if err != nil {
			t.Fatal(err)
		}
		r := mustCompileMultiClass(t, mod, "zcl_abcdefghijklmnopqrstuvwxyz", 1)
		if r.Stats.CrossChunkCalls != 1 || r.Stats.IndirectCalls != 1 {
			t.Fatal("fixture did not exercise both call paths")
		}
		for _, src := range r.ChunkClasses {
			for _, method := range regexp.MustCompile(`(?s)METHOD [a-z0-9_]+~f[0-9]+\.(.*?)ENDMETHOD\.`).FindAllStringSubmatch(src, -1) {
				types := map[string]string{}
				for _, decl := range regexp.MustCompile(`\b([a-z][a-z0-9_]*) TYPE (int8|i|f)\b`).FindAllStringSubmatch(method[1], -1) {
					types[decl[1]] = decl[2]
				}
				returns := regexp.MustCompile(`([a-z0-9_]+) = [a-z0-9_]+=>(?:go_c[0-9]+->f[0-9]+|dispatch_s[0-9]+)\(`).FindAllStringSubmatch(method[1], -1)
				for _, ret := range returns {
					if result == 0 || types[ret[1]] != result.ABAPType() {
						t.Fatalf("incorrect return type for %s", ret[1])
					}
				}
				for _, call := range regexp.MustCompile(`(?:->f[0-9]+|=>dispatch_s[0-9]+)\( ([^)]*)\)`).FindAllStringSubmatch(method[1], -1) {
					actuals := regexp.MustCompile(`p([0-9]+) = ([a-z0-9_]+)`).FindAllStringSubmatch(call[1], -1)
					if len(actuals) != len(ft.Params) {
						t.Fatalf("wrong call arity: %s", call[0])
					}
					for i, actual := range actuals {
						if types[actual[2]] != ft.Params[i].ABAPType() {
							t.Fatalf("%s has type %s, expected %s", actual[2], types[actual[2]], ft.Params[i].ABAPType())
						}
					}
				}
			}
		}
		for name := range r.Interfaces {
			if len(name) > 30 {
				t.Fatalf("long interface name %s", name)
			}
		}
		checkLineLimit(t, r.Files("zcl_abcdefghijklmnopqrstuvwxyz"))
	}
}
