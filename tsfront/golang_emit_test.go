package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
)

func runGoHIR(t *testing.T, p *hir.Program, main string, mutations ...func(map[string]string)) string {
	t.Helper()
	files, err := gohir.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range mutations {
		mutate(files)
	}
	dir := t.TempDir()
	files["go.mod"] = "module probe\n\ngo 1.26.0\n"
	files["main.go"] = "package main\nimport (\"fmt\";\"encoding/json\";\"os\")\nfunc main(){ _=json.Marshal;_=os.Args;" + main + "}\n"
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Go execution: %v\n%s\nmodule: %s", err, out, dir)
	}
	return string(out)
}
func goEntry(c, m string) string { return hir.NewNames().Get("body." + c + "." + m) }
func lexerCore(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	core := os.Getenv("TSFRONT_ABAPLINT")
	if core == "" {
		core = "/home/alice/dev/abaplint/packages/core"
	}
	build := filepath.Join(core, "build", "src")
	if _, err := os.Stat(filepath.Join(build, "abap", "1_lexer", "lexer.js")); err != nil {
		t.Skip("abaplint build absent; set TSFRONT_ABAPLINT")
	}
	return build
}
func TestGoLexerDifferential(t *testing.T) {
	core := lexerCore(t)
	cases, err := LoadLexerCorpus(filepath.Join("testdata", "lexercorpus"))
	if err != nil {
		t.Fatal(err)
	}
	oraclePath := filepath.Join(t.TempDir(), "tokens.json")
	cmd := exec.Command("node", "../tools/lexer-oracle.mjs", "testdata/lexercorpus/cases.json", oraclePath, core)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("oracle: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	var fresh []LexerCase
	if err := json.Unmarshal(raw, &fresh); err != nil {
		t.Fatal(err)
	}
	for i := range cases {
		if cases[i].Name != fresh[i].Name || cases[i].Dump != fresh[i].Dump {
			t.Fatalf("stale oracle: %s", cases[i].Name)
		}
	}
	p, ds := lowerClosure(t)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	var driver strings.Builder
	driver.WriteString("out:=[]string{};")
	for _, c := range cases {
		fmt.Fprintf(&driver, "out=append(out,%s(str(%q)).String());", goEntry("harness/lexer_dump.ts.LexerDump", "dump"), c.Abap)
	}
	driver.WriteString("b,_:=json.Marshal(out);fmt.Println(string(b))")
	out := runGoHIR(t, p, driver.String())
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err, out)
	}
	want := make([]string, len(cases))
	for i, c := range cases {
		want[i] = c.Dump
		if got[i] != c.Dump {
			t.Errorf("%s: got %q want %q", c.Name, got[i], c.Dump)
		}
	}
	if len(got) != 44 {
		t.Fatalf("got %d cases", len(got))
	}
	mutatedOut := runGoHIR(t, p, driver.String(), func(files map[string]string) {
		before := files["hir.go"]
		files["hir.go"] = strings.Replace(before, `str("Identifier")`, `str("MutatedIdentifier")`, 1)
		if files["hir.go"] == before {
			t.Fatal("mutation did not change emitted token type")
		}
	})
	var mutated []string
	if err := json.Unmarshal([]byte(mutatedOut), &mutated); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(mutated, want) {
		t.Fatal("emitted token type mutation escaped comparison")
	}
	t.Logf("%d/44 equal; token type mutation rejected", len(got))
}
