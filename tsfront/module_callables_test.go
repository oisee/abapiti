package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestModuleConstCallablesNodeOracle(t *testing.T) {
	source := `function add(a: number, b = 2, ...tail: number[]): number { return a + b + tail.length; }
 const alias = add;
 const again = alias;
 const title = (text: string): string => text.split("\n")[0];
 const double = (x: number): number => x * 2;
 class Probe { static run(): string { return title("first\nsecond") + ":" + again(3, undefined, 4, 5) + ":" + alias(1, 4) + ":" + [1,2].map(double).join(","); } }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err = abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestModuleCallableTDZNodeOracle(t *testing.T) {
	source := `class State {static effects=0;} function base(n:number):number{return n;} function bump():number{State.effects++;return 1;} late(bump()); const late=base;`
	// Node's TDZ lookup throws before the argument function is evaluated.
	want, err := exec.Command("node", "-e", `let effects=0;function bump(){effects++;return 1}try{late(bump())}catch(e){if(!(e instanceof ReferenceError))throw e}const late=n=>n;console.log(effects)`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err = abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	names := hir.NewNames()
	got := runGoHIR(t, p, fmt.Sprintf(`func(){defer func(){if _,ok:=recover().(trap);!ok{panic("missing TDZ refusal")}}();%s()}();fmt.Println(%s)`, names.Get("init.probe.ts.module"), names.Get("static.probe.ts.State.effects")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestRegistrationLiteralRowsNodeOracle(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := `const title = (text: string): string => text.split("\n")[0];
 const rows = [{n:"one\nline", expected:1}, {n:"two", expected:2, extra:true}];
 describe("rows",()=>{ rows.forEach(row=>{it(title(row.n),()=>{
   if (row.expected < 1) throw new Error("bad row");
   if (row.n === "one\\nline" && row.extra !== undefined) throw new Error("fabricated optional field");
   if (row.n === "two" && row.extra !== true) throw new Error("lost exclusive field");
 });}); });
 describe("local",()=>{const cases=[{input:"plain"}, {input:"configured", options:{flag:true}}];cases.forEach(row=>{it(row.input,()=>{
   if (row.input === "plain" && row.options !== undefined) throw new Error("fabricated options");
   if (row.input === "configured" && row.options?.flag !== true) throw new Error("lost options");
 });});});
 describe("later",()=>{it("independent",()=>{});});`
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": source})
	lowered, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, lowered)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed, Trapped int
		Results                 []struct{ Title string }
		Initialization          []any
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 5 || got.Failed != 0 || got.Trapped != 0 || len(got.Initialization) != 0 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
	// Independent Node registration oracle; the source is unchanged. This
	// small driver records ordinary describe/it nesting, not Go registrations.
	js := `const titles=[]; const suites=[]; function describe(t,f){suites.push(t);try{f()}finally{suites.pop()}} function it(t,f){titles.push([...suites,t].join(" "));f()}` + "\n" + source + "\nconsole.log(JSON.stringify(titles));"
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]))`, js).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	titles := []string{}
	for _, r := range got.Results {
		titles = append(titles, r.Title)
	}
	encoded, _ := json.Marshal(titles)
	if string(encoded) != strings.TrimSpace(string(want)) {
		t.Fatalf("Go %s Node %s", encoded, want)
	}
}
