package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestEmitStatementsClosure(t *testing.T) {
	if os.Getenv("STMTS_EMIT") == "" {
		t.Skip("set STMTS_EMIT=1 to emit full parser and differential")
	}
	prog, diags := lowerStmtsClosure(t)
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	assertSuperSameMethod(t, files)
	cases, err := LoadStatementsCorpus(filepath.Join("testdata", "stmtscorpus"))
	if err != nil {
		t.Fatal(err)
	}
	params := DriverParams{Class: names.Get("harness/statements_dump.ts.StatementsDump"), Dump: names.Get("member.dump"), Diff: names.Get("member.firstDiff"), Raw: names.Get("param.raw"), A: names.Get("param.a"), B: names.Get("param.b")}
	files[params.Class+".clas.testclasses.abap"] = StatementsTestClass(cases, params, names.Get("member.lastTokens"), names.Get("member.lastStatements"))
	regressions, err := LoadStatementsCorpus(filepath.Join("testdata", "stmtsregressions"))
	if err != nil {
		t.Fatal(err)
	}
	extra := StatementsTestClass(regressions, params, names.Get("member.lastTokens"), names.Get("member.lastStatements"))
	files[params.Class+".clas.testclasses.abap"] += strings.ReplaceAll(extra, "ltcl_statements_diff", "ltcl_statements_regressions")
	lines, bytes := 0, 0
	for name, source := range files {
		lines += strings.Count(source, "\n")
		bytes += len(source)
		for _, line := range strings.Split(source, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s: oversized line: %d", name, len(line))
			}
		}
	}
	t.Logf("emitted %d ABAP files, %d lines, %d bytes; %d differential cases", len(files), lines, bytes, len(cases))
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestStatementsDifferentialCorpus(t *testing.T) {
	cases, err := LoadStatementsCorpus(filepath.Join("testdata", "stmtscorpus"))
	if err != nil {
		t.Fatal(err)
	}
	tokens, statements := 0, 0
	for _, c := range cases {
		tokens += c.Tokens
		statements += c.Statements
	}
	if len(cases) != 64 || tokens != 5229 || statements != 926 {
		t.Fatalf("corpus changed: %d cases, %d tokens, %d statements", len(cases), tokens, statements)
	}
	driver := StatementsTestClass(cases, DriverParams{Class: "z_dump", Dump: "dump", Diff: "first_diff", Raw: "raw", A: "a", B: "b"}, "tokens", "statements")
	if strings.Count(driver, "METHODS statements FOR TESTING.") != 1 || strings.Count(driver, "CALL METHOD compare EXPORTING") != len(cases) {
		t.Fatal("driver must run every case in one test method")
	}
}
