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
			r := CompileMultiClass(mod, "zcl_split_test", budget)
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
				for _, m := range classMethodImplementation.FindAllStringSubmatch(src, -1) {
					seen[m[1]] = true
				}
				if lines := strings.Count(src, "\n"); lines > budget {
					t.Errorf("%s: %d lines exceeds %d", cls, lines, budget)
				}
			}
			for cls := range r.ChunkClasses {
				if strings.Contains(strings.ToLower(r.StateClass), cls) {
					t.Errorf("state mentions %s", cls)
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
				for _, bad := range []string{"DATA(", "NEW ", "|", "CONV ", "VALUE #", "zcl_wasm_rt", "PERFORM "} {
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
	r := CompileMultiClass(mod, "zcl_alias", 200)
	if !strings.Contains(r.MainClass, "METHOD alias_add.") || !strings.Contains(r.MainClass, "zcl_alias_c01=>f0(") {
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
	r := CompileMultiClass(mod, base, 40)
	if r.StateName != base+"_st" {
		t.Fatalf("state name %s", r.StateName)
	}
	for name := range r.ChunkClasses {
		if len(name) > 30 {
			t.Fatalf("class name too long: %s", name)
		}
	}
}
