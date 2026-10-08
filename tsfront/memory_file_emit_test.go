package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// A small native differential isolates the already lowered input file model
// from the still-blocked Registry and avoids the two large-input runtime gaps.
func TestEmitMemoryFileDifferential(t *testing.T) {
	if os.Getenv("MEMORY_FILE_EMIT") == "" {
		t.Skip("set MEMORY_FILE_EMIT=1 to emit the input file differential")
	}
	p, err := Load(filepath.Join(stmtsDir(t), "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower([]string{"src/files/_ifile.ts", "src/files/_abstract_file.ts", "src/files/memory_file.ts", "harness/memory_file_dump.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if os.Getenv("MEMORY_FILE_MUTATE") != "" {
		mutated := false
		for _, c := range prog.Classes {
			if c.Name != "src/files/_abstract_file.ts.AbstractFile" {
				continue
			}
			for _, m := range c.Methods {
				if m.Name == "getObjectName" {
					m.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(hir.T(hir.String), "MUTATED")})
					mutated = true
				}
			}
		}
		if !mutated {
			t.Fatal("MemoryFile mutant target missing")
		}
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Filename string `json:"filename"`
		Raw      string `json:"raw"`
		Dump     string `json:"dump"`
	}
	raw, err := os.ReadFile("testdata/registrycorpus/memory-dumps.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 18 {
		t.Fatal("missing MemoryFile oracle cases")
	}
	var inputs []struct {
		Filename string `json:"filename"`
		Raw      string `json:"raw"`
	}
	raw, err = os.ReadFile("testdata/registrycorpus/memory-files.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &inputs); err != nil {
		t.Fatal(err)
	}
	if len(inputs) != len(cases) {
		t.Fatal("MemoryFile input/oracle length mismatch")
	}
	for idx, c := range cases {
		if c.Filename != inputs[idx].Filename || c.Raw != inputs[idx].Raw {
			t.Fatalf("MemoryFile input/oracle mismatch at case %d", idx)
		}
	}
	class := names.Get("harness/memory_file_dump.ts.MemoryFileDump")
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("CLASS ltcl_memory_file DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	line("METHODS files FOR TESTING.")
	line("METHODS splits FOR TESTING.")
	line("ENDCLASS.")
	line("CLASS ltcl_memory_file IMPLEMENTATION.")
	line("METHOD files.")
	for _, name := range []string{"filename", "raw", "expected", "actual"} {
		line("DATA %s TYPE string.", name)
	}
	line("DATA ch TYPE c LENGTH 1.")
	for idx, c := range cases {
		for _, field := range []struct{ name, value string }{{"filename", c.Filename}, {"raw", c.Raw}, {"expected", c.Dump}} {
			line("CLEAR %s.", field.name)
			for _, s := range abapStringBuild(field.name, "ch", field.value) {
				line("%s", s)
			}
		}
		line("CALL METHOD %s=>%s EXPORTING %s = filename %s = raw RECEIVING result = actual.", class, names.Get("member.dump"), names.Get("param.filename"), names.Get("param.raw"))
		line("cl_abap_unit_assert=>assert_equals( act = actual exp = expected msg = `MemoryFile case %d` ).", idx+1)
	}
	line("ENDMETHOD.")
	var splits []struct {
		Raw       string `json:"raw"`
		Separator string `json:"separator"`
		Dump      string `json:"dump"`
	}
	raw, err = os.ReadFile("testdata/registrycorpus/split-dumps.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &splits); err != nil {
		t.Fatal(err)
	}
	if len(splits) != 16 {
		t.Fatal("missing string.split oracle cases")
	}
	line("METHOD splits.")
	for _, name := range []string{"raw", "separator", "expected", "actual"} {
		line("DATA %s TYPE string.", name)
	}
	line("DATA ch TYPE c LENGTH 1.")
	for idx, c := range splits {
		for _, field := range []struct{ name, value string }{{"raw", c.Raw}, {"separator", c.Separator}, {"expected", c.Dump}} {
			line("CLEAR %s.", field.name)
			for _, s := range abapStringBuild(field.name, "ch", field.value) {
				line("%s", s)
			}
		}
		line("CALL METHOD %s=>%s EXPORTING %s = raw %s = separator RECEIVING result = actual.", class, names.Get("member.dumpSplit"), names.Get("param.raw"), names.Get("param.separator"))
		line("cl_abap_unit_assert=>assert_equals( act = actual exp = expected msg = `string.split case %d` ).", idx+1)
	}
	line("ENDMETHOD.")
	line("ENDCLASS.")
	files[class+".clas.testclasses.abap"] = b.String()
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("emitted %d files, %d MemoryFile differential cases", len(files), len(cases))
}
