package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func gogenOracle(t *testing.T, source, expected string) {
	t.Helper()
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	gogenCheck(t, p, "probe.ts", source, expected)
}

func gogenCheck(t *testing.T, p *hir.Program, file, source, expected string) {
	t.Helper()
	if t.Failed() {
		return
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry(file+".Probe", "run")))
	if got != expected+"\n" {
		t.Fatalf("Go %q want %q", got, expected)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err == nil {
		want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(want))
		}
		if got != string(want) {
			t.Fatalf("Go %q Node %q", got, want)
		}
	} else {
		t.Log("Node absent; Go and ABAP checks still ran")
	}
}

func TestGogenJSDocObjectParameters(t *testing.T) {
	source := `/** @typedef {{expr: string, place: string}} Renderers */
 /** @param {number} st @param {number} ctx @param {number} t @param {Renderers} renderers @returns {string} */
 function f(st, ctx, t, {expr, place}) {return st + ctx + t + expr + place;}
 /** @param {string} program @param {{goType:string, ident:string, classes:string, poly:string, ancestorsOf:string}} helpers @returns {string} */
 function g(program, {goType, ident, classes: CLASSES, poly: POLY, ancestorsOf}) {return program + goType + ident + CLASSES + POLY + ancestorsOf;}
 class Probe {/** @returns {string} */ static run() {return f(1,2,3,{expr:"e",place:"p"})+":"+g("p",{goType:"g",ident:"i",classes:"C",poly:"P",ancestorsOf:"a"});}}`
	dir := t.TempDir()
	for file, content := range map[string]string{
		"probe.js":      source,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","allowJs":true,"checkJs":true,"noEmit":true},"files":["probe.js"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	project, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, diags, err := project.Lower([]string{"probe.js"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Error(d)
		}
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	gogenCheck(t, p, "probe.js", source, "6ep:pgiCPa")
}

func TestGogenObjectParameters(t *testing.T) {
	gogenOracle(t, `interface ByteRenderers {expr: string; place: string; rowValue: number;}
 function f(st: string, ctx: string, t: number, {expr, place, rowValue}: ByteRenderers): string {
  expr = expr + "!";
  return st + ctx + t + expr + place + rowValue;
 }
 function g(program: string, {classes: CLASSES, poly: POLY}: {classes: string; poly: string}): string { return program + CLASSES + POLY; }
 class Probe { static run(): string { const r = {expr:"e", place:"p", rowValue:3}; return f("s","c",2,r) + ":" + r.expr + ":" + g("g",{classes:"C",poly:"P"}); } }`, "sc2e!p3:e:gCP")
}

func TestGogenObjectParameterGetters(t *testing.T) {
	gogenOracle(t, `class Source {
 static trace="";
 get expr(): string {Source.trace += "e";return Source.trace;}
 get place(): string {Source.trace += "p";return Source.trace;}
 static arg(): number {Source.trace += "a";return 1;}
 }
 function read({expr, place: renamed}: Source, n: number): string {return expr+":"+renamed+":"+Source.trace+n;}
 class Probe {static run(): string {return read(new Source(),Source.arg());}}`, "ae:aep:aep1")
}

func TestGogenObjectParameterRefusals(t *testing.T) {
	for _, pattern := range []string{"{a = 1}", "{...rest}", "{a: {b}}"} {
		t.Run(pattern, func(t *testing.T) {
			requireDiagnostic(t, sourceProbe(t, "function f("+pattern+": any):void {}"), "unsupported-param")
		})
	}
}
