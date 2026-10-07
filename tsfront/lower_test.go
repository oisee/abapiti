package tsfront

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// closureFiles are the vendored lexer closure plus abapiti's driver, in a
// deterministic order.
var closureFiles = []string{
	"src/position.ts",
	"src/virtual_position.ts",
	"src/files/_ifile.ts",
	"src/abap/1_lexer/lexer_result.ts",
	"src/abap/1_lexer/lexer_buffer.ts",
	"src/abap/1_lexer/lexer_stream.ts",
	"src/abap/1_lexer/tokens/abstract_token.ts",
	"harness/test_file.ts",
	"harness/token_name.ts",
	"harness/lexer_dump.ts",
	"src/abap/1_lexer/lexer.ts",
}

func closureTokens() []string {
	entries, err := os.ReadDir(filepath.Join("testdata", "lexer", "src", "abap", "1_lexer", "tokens"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.Name() == "index.ts" || e.Name() == "abstract_token.ts" {
			continue
		}
		out = append(out, "src/abap/1_lexer/tokens/"+e.Name())
	}
	sort.Strings(out)
	return out
}

func lowerClosure(t *testing.T) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	p, err := Load(filepath.Join("testdata", "lexer", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	files := append([]string{}, closureFiles[:9]...)
	files = append(files, closureTokens()...)
	files = append(files, closureFiles[9], closureFiles[10])
	prog, diags, err := p.Lower(files)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	return prog, diags
}

func TestLowerLexerClosure(t *testing.T) {
	prog, diags := lowerClosure(t)
	byCategory := map[string]int{}
	for _, d := range diags {
		byCategory[d.Category]++
		if !strings.HasPrefix(d.Category, "note-") && d.Category != "skipped-computed-name" {
			t.Errorf("blocking diagnostic: %s", d)
		}
	}
	// The closure must lower completely: only policy notes and the one
	// skipped debug method are allowed.
	want := map[string]int{
		"skipped-computed-name": 1, // [Symbol.for("debug.description")] in AbstractToken
		"note-number-i32":       39,
		"note-optional-param":   1,
		"note-regex-mapped":     1,
		"note-union-base":       1,
	}
	if !reflect.DeepEqual(byCategory, want) {
		t.Errorf("diagnostic categories %v, want %v", byCategory, want)
	}
	if t.Failed() {
		for _, c := range sortedKeys(byCategory) {
			t.Logf("%s: %d", c, byCategory[c])
		}
		return
	}
	if errors := hir.Verify(prog); len(errors) > 0 {
		for _, e := range errors {
			t.Errorf("verify: %v", e)
		}
	}
	// 47 concrete token classes + AbstractToken + Lexer + LexerBuffer +
	// LexerStream + Position + VirtualPosition + TestFile + TokenName +
	// LexerDump + 2 module classes + the synthesized IABAPLexerResult.
	classes := len(prog.Classes)
	if classes != 59 {
		var names []string
		for _, c := range prog.Classes {
			names = append(names, c.Name)
		}
		t.Errorf("expected 59 lowered classes, got %d: %v", classes, names)
	}
	t.Logf("%d classes, %d interfaces, %d diagnostics", classes, len(prog.Interfaces), len(diags))
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(out, "lexer.hir.txt"), []byte(hir.Dump(prog)), 0o644)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	var lines int
	for name, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Errorf("%s: line of %d bytes", name, len(line))
			}
		}
		lines += strings.Count(src, "\n")
	}
	t.Logf("emitted %d files, %d lines", len(files), lines)

	// Attach the differential unit test to the lowered driver class: run the
	// lowered lexer on every corpus case and compare with the oracle dumps
	// produced by the original TypeScript lexer.
	cases, err := LoadLexerCorpus(filepath.Join("testdata", "lexercorpus"))
	if err != nil {
		t.Fatal(err)
	}
	params := DriverParams{
		Class: names.Get("harness/lexer_dump.ts.LexerDump"),
		Dump:  names.Get("member.dump"),
		Diff:  names.Get("member.firstDiff"),
		Raw:   names.Get("param.raw"),
		A:     names.Get("param.a"),
		B:     names.Get("param.b"),
	}
	test := LexerTestClass(cases, params)
	for _, line := range strings.Split(test, "\n") {
		if len(line) > 255 {
			t.Errorf("driver: line of %d bytes", len(line))
		}
	}
	files[params.Class+".clas.testclasses.abap"] = test
	t.Logf("differential driver: %d cases, %d lines", len(cases), strings.Count(test, "\n"))
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		for name, src := range files {
			os.WriteFile(filepath.Join(out, name), []byte(src), 0o644)
		}
	}
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
