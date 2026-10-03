package wasm

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func collidingNameModule(t *testing.T) *Module {
	t.Helper()
	names := []string{
		"probe_reinterpret64_negative_zero_lo", "probe_reinterpret64_negative_zero_hi",
		strings.Repeat("a", 30) + "first", strings.Repeat("a", 30) + "second",
		"CaseOnly", "caseonly", "punct-name", "punct.name", "punct$name",
		"constructor", "mem_ld_i32", "dispatch_t0", "f13", "",
		strings.Repeat("a", 28) + "_1", "123 invalid/é",
	}
	var exports []Export
	var bodies [][]byte
	for i, name := range names {
		if name != "" {
			exports = append(exports, Export{Name: name, Kind: 0, Index: i})
		}
		code := append([]byte{OpI32Const}, leb128s(int32(i))...)
		// Exercise calls to the allocated collision names, with distinct bodies.
		if i > 0 {
			code = append(code, OpDrop, OpCall)
			code = append(code, leb128u(uint32(i-1))...)
		}
		bodies = append(bodies, buildFuncBody(nil, code))
	}
	w := newWasmBuilder()
	w.addSection(1, buildTypeSection([]FuncType{{Results: []ValType{ValI32}}}))
	w.addSection(3, buildFuncSection(make([]int, len(names))))
	w.addSection(7, buildExportSection(exports))
	w.addSection(10, buildCodeSection(bodies))
	mod, err := Parse(w.bytes())
	if err != nil {
		t.Fatal(err)
	}
	return mod
}

func checkNamedMethods(t *testing.T, src string, names []string, implementations bool) {
	t.Helper()
	declarations := regexp.MustCompile(`(?mi)^\s*(?:CLASS-)?METHODS ([a-z0-9_]+)\b`).FindAllStringSubmatch(src, -1)
	bodies := regexp.MustCompile(`(?mi)^\s*METHOD ([a-z0-9_]+)\.`).FindAllStringSubmatch(src, -1)
	for _, matches := range [][][]string{declarations, bodies} {
		seen := map[string]bool{}
		for _, m := range matches {
			name := strings.ToLower(m[1])
			if seen[name] {
				t.Errorf("duplicate method %s", name)
			}
			seen[name] = true
		}
	}
	for _, name := range names {
		count := func(matches [][]string) int {
			n := 0
			for _, m := range matches {
				if strings.EqualFold(m[1], name) {
					n++
				}
			}
			return n
		}
		if count(declarations) != 1 {
			t.Errorf("%s declarations = %d", name, count(declarations))
		}
		if implementations && count(bodies) != 1 {
			t.Errorf("%s implementations = %d", name, count(bodies))
		}
	}
}

func TestUniqueNamesAllBackends(t *testing.T) {
	mod := collidingNameModule(t)
	names := moduleFunctionNames(mod)
	if !reflect.DeepEqual(names, moduleFunctionNames(mod)) {
		t.Fatal("unstable allocation")
	}
	seen := map[string]bool{}
	valid := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,29}$`)
	var exports []string
	for i, name := range names {
		if !valid.MatchString(name) || seen[strings.ToLower(name)] {
			t.Fatalf("invalid or duplicate name %q", name)
		}
		seen[strings.ToLower(name)] = true
		if mod.Functions[i].ExportName != "" {
			exports = append(exports, name)
		}
	}
	for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
		t.Run(backend.String(), func(t *testing.T) {
			result := CompileWith(mod, "zcl_collision", backend, 3)
			if !reflect.DeepEqual(result.Files, CompileWith(mod, "zcl_collision", backend, 3).Files) {
				t.Fatal("unstable source")
			}
			checkLineLimit(t, result.Files)
			if backend != BackendFUGR {
				checkNamedMethods(t, result.Files["zcl_collision.clas.abap"], exports, true)
			}
			if backend == BackendClass {
				src := result.Files["zcl_collision.clas.abap"]
				for _, name := range names[:len(names)-1] {
					if !strings.Contains(src, name+"( )") {
						t.Errorf("missing call to %s", name)
					}
				}
				return
			}
			prefix := "zcl_collision"
			if backend == BackendHybrid {
				prefix += "_int"
			}
			fms := functionModuleNames(mod, prefix)
			all := ""
			for _, src := range result.Files {
				all += src + "\n"
			}
			for i, name := range names {
				re := regexp.MustCompile(`(?mi)^FORM ` + name + `\b`)
				if n := len(re.FindAllString(all, -1)); n != 1 {
					t.Errorf("%s FORM count = %d", name, n)
				}
				if i < len(names)-1 && !strings.Contains(all, "PERFORM "+name+" CHANGING") {
					t.Errorf("missing call to %s", name)
				}
				if mod.Functions[i].ExportName == "" {
					continue
				}
				fm := fms[i]
				if len(fm) > 30 {
					t.Errorf("long FM %s", fm)
				}
				wrapper, ok := result.Files[fm+".func.abap"]
				if !ok || !strings.Contains(wrapper, "PERFORM "+name+" CHANGING") {
					t.Errorf("missing wrapper for %s", name)
				}
				if backend == BackendHybrid && !strings.Contains(result.Files["zcl_collision.clas.abap"], "CALL FUNCTION '"+strings.ToLower(fm)+"'") {
					t.Errorf("missing hybrid call to %s", fm)
				}
			}
		})
	}
	t.Run("multi-class", func(t *testing.T) {
		for _, size := range []int{3, 80} {
			result := CompileMultiClass(mod, "zcl_collision", size)
			checkNamedMethods(t, result.MainClass, exports, true)
			for chunk, src := range result.ChunkClasses {
				var expected []string
				for i, name := range names {
					if chunk == fmt.Sprintf("zcl_collision_c%02d", i/size) {
						expected = append(expected, name)
					}
				}
				checkNamedMethods(t, src, expected, true)
				for _, name := range expected {
					if mod.Functions[indexOf(names, name)].ExportName != "" && !strings.Contains(result.MainClass, "->"+name+"( )") {
						t.Errorf("missing delegation to %s", name)
					}
				}
			}
		}
	})
	t.Run("interface", func(t *testing.T) {
		src := CompileInterface(mod, "zif_collision")
		checkNamedMethods(t, src, exports, false)
		if src != CompileInterface(mod, "zif_collision") {
			t.Fatal("unstable interface")
		}
		checkLineLimit(t, map[string]string{"interface": src})
	})
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

func TestUniqueNamesIndirectDispatch(t *testing.T) {
	mod, err := Parse(buildCallIndirectModule())
	if err != nil {
		t.Fatal(err)
	}
	mod.Functions[0].ExportName = "probe_reinterpret64_negative_zero_lo"
	mod.Functions[1].ExportName = "probe_reinterpret64_negative_zero_hi"
	names := moduleFunctionNames(mod)
	src := Compile(mod, "zcl_dispatch_collision")
	checkNamedMethods(t, src, []string{names[0], names[1]}, true)
	dispatch := src[strings.Index(src, "METHOD dispatch_t0."):]
	dispatch = dispatch[:strings.Index(dispatch, "ENDMETHOD.")]
	for _, name := range names[:2] {
		if !strings.Contains(dispatch, name+"(") {
			t.Errorf("missing indirect target %s", name)
		}
	}
}

func TestUniqueNamesDeduplicatedDelegates(t *testing.T) {
	mod := collidingNameModule(t)
	mod.Functions[1].Code = mod.Functions[0].Code
	names := moduleFunctionNames(mod)
	redirects := DeduplicateFunctions(mod)
	if redirects[1] != 0 {
		t.Fatal("fixture must redirect second function to first")
	}
	for _, backend := range []BackendKind{BackendFUGR, BackendHybrid} {
		result := CompileWith(mod, "zcl_dupe", backend, 3)
		prefix := "zcl_dupe"
		if backend == BackendHybrid {
			prefix += "_int"
		}
		fms := functionModuleNames(mod, prefix)
		if !strings.Contains(result.Files[fms[1]+".func.abap"], "PERFORM "+names[0]+" CHANGING") {
			t.Errorf("%s duplicate wrapper does not call canonical function", backend)
		}
	}
	result := CompileMultiClass(mod, "zcl_dupe", 3)
	checkNamedMethods(t, result.MainClass, names[:2], true)
	for _, name := range names[:2] {
		body := result.MainClass[strings.Index(result.MainClass, "METHOD "+name+"."):]
		body = body[:strings.Index(body, "ENDMETHOD.")]
		if !strings.Contains(body, "mo_c00->"+names[0]+"( )") {
			t.Errorf("%s does not delegate to canonical method", name)
		}
	}
}
