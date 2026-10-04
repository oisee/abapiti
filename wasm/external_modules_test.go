package wasm

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestExternalModules needs locally built modules; it never downloads anything.
// ABAPITI_EXTERNAL_DIR overrides testdata/external/wasm. ABAPITI_TEST_OUT
// exports the modules, generated ABAP, and tests expecting the native answers.
func TestExternalModules(t *testing.T) {
	dir := os.Getenv("ABAPITI_EXTERNAL_DIR")
	if dir == "" {
		dir = "testdata/external/wasm"
	}
	for _, fixture := range []struct {
		dir, module string
		checks      int
		split       bool
	}{
		{"monocypher", "mono", 10, false},
		{"quickjs", "quickjs", 16, true},
	} {
		t.Run(fixture.dir, func(t *testing.T) {
			path := filepath.Join(dir, fixture.module+".wasm")
			bin, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				t.Skipf("external module %s is missing; run testdata/external/%s/build.sh or set ABAPITI_EXTERNAL_DIR to the module output directory", path, fixture.dir)
			}
			if err != nil {
				t.Fatal(err)
			}
			dataDir := filepath.Join("testdata/external", fixture.dir)
			cases, want := externalChecks(t, dataDir, fixture.module, fixture.checks)
			got := wazeroResults(t, bin, cases)
			for i, c := range cases {
				if got[i] != want[i] {
					t.Fatalf("%s %v: wazero %+v, native %+v", c.fn, c.args, got[i], want[i])
				}
				t.Logf("%s %v = %d", c.fn, c.args, got[i].value)
			}
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			class := "zcl_abapiti_t_" + fixture.module
			var files map[string]string
			if fixture.split {
				result, err := CompileMultiClass(mod, class, DefaultClassLines)
				if err != nil {
					t.Fatal(err)
				}
				files, err = result.Files(class)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("ABAP split: %d chunks, %d interfaces, max-lines=%d", len(result.ChunkClasses), len(result.Interfaces), result.Stats.MaxClassLines)
			} else {
				files = map[string]string{class + ".clas.abap": mustCompile(t, mod, class)}
			}
			if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
				out = filepath.Join(out, "TestExternalModules", fixture.dir)
				if err := os.MkdirAll(out, 0o755); err != nil {
					t.Fatal(err)
				}
				files[fixture.module+".wasm"] = string(bin)
				files[class+".clas.testclasses.abap"] = osdTestClass(class, cases, want)
				for name, src := range files {
					if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("exported %d files to %s", len(files), out)
			}
		})
	}
}

func externalChecks(t *testing.T, dir, module string, count int) ([]osdCase, []osdResult) {
	t.Helper()
	read := func(name string) []string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
	native := map[string]int32{}
	for _, line := range read("native-results.txt") {
		key, value, ok := strings.Cut(line, " = ")
		answer, err := strconv.ParseInt(value, 10, 32)
		if !ok || err != nil {
			t.Fatalf("invalid native answer %q", line)
		}
		if _, exists := native[key]; exists {
			t.Fatalf("duplicate native answer %q", key)
		}
		native[key] = int32(answer)
	}
	var cases []osdCase
	var want []osdResult
	seen := map[string]bool{}
	for _, line := range read("cases.txt") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != module {
			t.Fatalf("invalid case %q", line)
		}
		c := osdCase{fn: fields[1]}
		key := strings.Join(fields, " ")
		if seen[key] {
			t.Fatalf("duplicate case %q", key)
		}
		seen[key] = true
		for _, arg := range fields[2:] {
			v, err := strconv.ParseInt(arg, 10, 32)
			if err != nil {
				t.Fatalf("%s: %v", key, err)
			}
			c.args = append(c.args, int32(v))
		}
		answer, ok := native[key]
		if !ok {
			t.Fatalf("no native answer for %s", key)
		}
		cases = append(cases, c)
		want = append(want, osdResult{value: answer})
	}
	if len(cases) != count || len(native) != count {
		t.Fatalf("%s: %d cases and %d answers, want %d each", module, len(cases), len(native), count)
	}
	return cases, want
}
