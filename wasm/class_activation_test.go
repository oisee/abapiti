package wasm

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var classMethodCall = regexp.MustCompile(`\b(?:me->)?([a-z][a-z0-9_]*)\s*\(`)
var classMethodDefinition = regexp.MustCompile(`(?m)^\s*METHODS\s+([a-z][a-z0-9_]*)\b`)
var classMethodImplementation = regexp.MustCompile(`(?m)^\s*METHOD\s+([a-z][a-z0-9_]*)\.`)
var parameterWrite = regexp.MustCompile(`(?:^|\.)\s*p[0-9]+\s*=`)

func compileCFixture(t *testing.T, sources ...string) []byte {
	t.Helper()
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang is missing")
	}
	if _, err := exec.LookPath("wasm-ld"); err != nil {
		t.Skip("wasm-ld is missing")
	}
	out := filepath.Join(t.TempDir(), "module.wasm")
	args := []string{"--target=wasm32", "-O2", "-nostdlib", "-Wl,--no-entry", "-Wl,--export-all", "-Wl,--allow-undefined", "-o", out}
	args = append(args, sources...)
	cmd := exec.Command(clang, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clang: %v\n%s", err, output)
	}
	bin, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return bin
}

func classFixtureModules(t *testing.T) map[string][]byte {
	t.Helper()
	modules := make(map[string][]byte)
	files, err := filepath.Glob("testdata/*.wasm")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		bin, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		modules[strings.TrimSuffix(filepath.Base(file), ".wasm")] = bin
	}
	modules["corpus"] = compileCFixture(t, "../llvm/testdata/corpus.c")
	if dir := os.Getenv("MONOCYPHER_SRC_DIR"); dir != "" {
		source := filepath.Join(dir, "monocypher.c")
		mem := filepath.Join(dir, "..", "mem.c")
		if _, err := os.Stat(mem); err != nil {
			mem = filepath.Join(dir, "..", "..", "mem.c")
		}
		modules["monocypher"] = compileCFixture(t, source, mem)
	} else {
		t.Run("monocypher_source", func(t *testing.T) {
			t.Skip("MONOCYPHER_SRC_DIR is unset")
		})
	}
	return modules
}

func TestClassOutputInvariants(t *testing.T) {
	modules := classFixtureModules(t)
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			mod, err := Parse(modules[name])
			if err != nil {
				t.Fatal(err)
			}
			src := Compile(mod, "zcl_wasm_"+name)
			if outDir := os.Getenv("ABAPITI_CLASS_OUT"); outDir != "" {
				if err := os.MkdirAll(outDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outDir, name+".clas.abap"), []byte(src), 0644); err != nil {
					t.Fatal(err)
				}
			}
			declared := make(map[string]bool)
			for _, match := range classMethodDefinition.FindAllStringSubmatch(src, -1) {
				declared[match[1]] = true
			}
			implemented := make(map[string]bool)
			for _, match := range classMethodImplementation.FindAllStringSubmatch(src, -1) {
				implemented[match[1]] = true
			}
			implementation := src[strings.Index(src, " IMPLEMENTATION."):]
			builtins := map[string]bool{"ipow": true, "abs": true, "trunc": true, "xstrlen": true, "xsdbool": true, "strlen": true, "lines": true, "min": true, "max": true, "condense": true}
			for _, name := range []string{"i", "int8", "f", "floor", "ceil", "sqrt", "round", "xstring"} {
				builtins[name] = true
			}
			missing := make(map[string]bool)
			for _, match := range classMethodCall.FindAllStringSubmatch(implementation, -1) {
				name := match[1]
				if strings.HasPrefix(name, "iv_") || strings.HasPrefix(name, "lv_") || strings.HasPrefix(name, "mv_") {
					continue
				}
				if (!declared[name] || !implemented[name]) && !builtins[name] && !missing[name] {
					missing[name] = true
					t.Errorf("undefined method %s", name)
				}
			}
			if strings.Contains(src, "zcl_wasm_rt=>") {
				t.Error("external runtime call")
			}
			if parameterWrite.MatchString(implementation) {
				t.Error("assignment to importing parameter")
			}
			maxLen := 0
			for _, line := range strings.Split(src, "\n") {
				if strings.Contains(line, "\"") || strings.HasPrefix(strings.TrimSpace(line), "*") {
					t.Errorf("ABAP comment: %s", line)
				}
				if len(line) > maxLen {
					maxLen = len(line)
				}
			}
			if maxLen > 255 {
				t.Errorf("max line length %d", maxLen)
			}
			t.Logf("lines=%d max=%d comments=0 helpers=%d", strings.Count(src, "\n"), maxLen, len(runtimeNamesInSource(src)))
		})
	}
}

func TestAllBackendOutputHasNoComments(t *testing.T) {
	for name, bin := range classFixtureModules(t) {
		t.Run(name, func(t *testing.T) {
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []BackendKind{BackendFUGR, BackendHybrid} {
				result := CompileWith(mod, "zcl_comment_test", backend, 80)
				for file, src := range result.Files {
					assertNoABAPComments(t, file, src)
				}
			}
			multi := CompileMultiClass(mod, "zcl_comment_test", 80)
			assertNoABAPComments(t, "main", multi.MainClass)
			assertNoABAPComments(t, "state", multi.StateClass)
			for file, src := range multi.ChunkClasses {
				assertNoABAPComments(t, file, src)
			}
		})
	}
}

func assertNoABAPComments(t *testing.T, file, src string) {
	t.Helper()
	for lineNo, line := range strings.Split(src, "\n") {
		if stripABAPComment(line) != line || strings.HasPrefix(strings.TrimSpace(line), "*") {
			t.Errorf("%s:%d: ABAP comment: %s", file, lineNo+1, line)
		}
	}
}

func runtimeNamesInSource(src string) []string {
	defs, _ := runtimeTemplates()
	var names []string
	for name := range defs {
		if strings.Contains(src, "METHOD "+name+".") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
