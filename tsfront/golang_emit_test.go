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
		files["hir.go"] = strings.ReplaceAll(before, `str("Identifier")`, `str("MutatedIdentifier")`)
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

// Reuse precisely the HIR and pinned overrides used by the ABAP fixture tests.
func goRegistryOracle(t *testing.T, p *hir.Program, fixture, class string) {
	t.Helper()
	lexerCore(t)
	raw, err := os.ReadFile("testdata/registryfeatures/" + fixture + "-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle map[string]json.RawMessage
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	var main strings.Builder
	main.WriteString("out:=map[string]string{};")
	for _, c := range p.Classes {
		if c.Name != fixture+".ts."+class {
			continue
		}
		for _, m := range c.Methods {
			var expected string
			if !m.Static || len(m.Params) > 0 || m.Result.Kind != hir.String || json.Unmarshal(oracle[m.Name], &expected) != nil {
				continue
			}
			fmt.Fprintf(&main, "out[%q]=%s().String();", m.Name, goEntry(c.Name, m.Name))
		}
	}
	if fixture == "sorts" {
		fmt.Fprintf(&main, "func(){defer func(){if recover()==nil {panic(\"ordering domain accepted\")}}();%s();}();", goEntry("sorts.ts.SortProbe", "rejected"))
	}
	main.WriteString("b,_:=json.Marshal(out);fmt.Println(string(b))")
	out := runGoHIR(t, p, main.String())
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err, out)
	}
	if len(got) == 0 {
		t.Fatal("no oracle methods executed")
	}
	for name, actual := range got {
		var expected string
		json.Unmarshal(oracle[name], &expected)
		if actual != expected {
			t.Errorf("%s: got %q want %q", name, actual, expected)
		}
	}
	t.Logf("%d Node oracle methods compared", len(got))
}

func TestGoRegistryJSON(t *testing.T) {
	lexerCore(t)
	p := lowerRegistryJSON(t)
	raw, err := os.ReadFile("testdata/registryfeatures/config-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Input, Expected string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	main := "out:=[]string{};"
	for _, c := range cases {
		main += fmt.Sprintf("out=append(out,%s(str(%q)).String());", goEntry("json.ts.JSONProbe", "defaults"), c.Input)
	}
	main += "b,_:=json.Marshal(out);fmt.Println(string(b))"
	out := runGoHIR(t, p, main)
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err, out)
	}
	for i, c := range cases {
		if got[i] != c.Expected {
			t.Errorf("case %d: got %q want %q", i, got[i], c.Expected)
		}
	}
	t.Logf("%d config Node oracle cases compared", len(cases))
}

func TestGoRegistryXML(t *testing.T) {
	lexerCore(t)
	p := lowerRegistryXML(t)
	raw, err := os.ReadFile("testdata/registryfeatures/tagged-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Flag     bool
		Expected string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	main := "out:=[]string{};"
	for _, c := range cases {
		main += fmt.Sprintf("out=append(out,%s(%t).String());", goEntry("xml.ts.XMLProbe", "tagged"), c.Flag)
	}
	main += "b,_:=json.Marshal(out);fmt.Println(string(b))"
	out := runGoHIR(t, p, main)
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err, out)
	}
	for i, c := range cases {
		if got[i] != c.Expected {
			t.Errorf("case %d: got %q want %q", i, got[i], c.Expected)
		}
	}
	t.Logf("%d tagged XML Node oracle cases compared", len(cases))
}

// tools/lexer-go-timing.mjs builds once, then times both runtimes in one process.
func TestPrepareGoLexerTiming(t *testing.T) {
	target := os.Getenv("ABAPITI_GO_LEXER_BENCH")
	if target == "" {
		t.Skip("run tools/lexer-go-timing.mjs for Go/Node timings")
	}
	lexerCore(t)
	p, ds := lowerClosure(t)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	files, err := gohir.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	var result hir.Type
	for _, c := range p.Classes {
		if c.Name == "src/abap/1_lexer/lexer.ts.Lexer" {
			for _, m := range c.Methods {
				if m.Name == "run" {
					result = m.Result
				}
			}
		}
	}
	if result.Name == "" {
		t.Fatal("missing Lexer.run")
	}
	n := hir.NewNames()
	invocation := fmt.Sprintf("%s(%s(),%s(raw),nil).%s().%s.Items", goEntry("src/abap/1_lexer/lexer.ts.Lexer", "run"), n.Get("new.src/abap/1_lexer/lexer.ts.Lexer"), n.Get("new.harness/test_file.ts.TestFile"), n.Get("base."+result.Name), n.Get("member.tokens"))
	files["main.go"] = `package main
import("encoding/json";"os";"time";"fmt";"runtime";"runtime/pprof")
func main(){if path:=os.Getenv("ABAPITI_LEXER_CPU_PROFILE");path!=""{f,err:=os.Create(path);if err!=nil{panic(err)};if err=pprof.StartCPUProfile(f);err!=nil{panic(err)};defer func(){pprof.StopCPUProfile();f.Close()}()};if path:=os.Getenv("ABAPITI_LEXER_ALLOC_PROFILE");path!=""{defer func(){f,err:=os.Create(path);if err!=nil{panic(err)};runtime.GC();if err=pprof.Lookup("allocs").WriteTo(f,0);err!=nil{panic(err)};f.Close()}()};var request struct {Cases []struct{Name,Abap string};Iterations int};if err:=json.NewDecoder(os.Stdin).Decode(&request);err!=nil{panic(err)}
 type observation struct{Name string ` + "`json:\"name\"`" + `;Milliseconds float64 ` + "`json:\"milliseconds\"`" + `;Tokens int ` + "`json:\"tokens\"`" + `}
 out:=[]observation{};for _,c:=range request.Cases{raw:=str(c.Abap);lex:=func()int{return len(` + invocation + `)};for i:=0;i<30;i++{lex()};start:=time.Now();count:=0;for i:=0;i<request.Iterations;i++{count=lex()};elapsed:=float64(time.Since(start).Nanoseconds())/1e6/float64(request.Iterations);out=append(out,observation{c.Name,elapsed,count})};b,_:=json.Marshal(out);fmt.Println(string(b))}
`
	files["go.mod"] = "module timing\n\ngo 1.26.0\n"
	dir := t.TempDir()
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "-o", target, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build timing driver: %v\n%s", err, out)
	}
}
