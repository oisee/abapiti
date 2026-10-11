package tsfront

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestRegistrationLocalHelpersNodeOracle(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := `describe("helpers", () => {
   function field(name: string): string { return "<field>" + name + "</field>"; }
   function table(name: string, fields: string[]): string { return name + ":" + fields.join("|"); }
   const suffix = "!";
   function title(name: string, extra: string = suffix): string { return name + extra; }
   function nested(name: string): void { describe(title(name), () => it("inside", () => {})); }
   const fixtures = [table("one", [field("A"), field("B")]), table("two", [field("C")])];
   fixtures.forEach(value => it(value, () => { if (value.length < 1) throw new Error("empty fixture"); }));
   nested("nested");
   const upper = (text: string): string => text.toUpperCase();
   it("immutable arrow", () => { if (upper("value") !== "VALUE") throw new Error("wrong arrow"); });
   function select(index: number): string { return fixtures[index]; }
   function first(): string { return select(0); }
   it("live fixture", () => {
     if (first() !== fixtures[0]) throw new Error("wrong capture");
     fixtures[0] = "mutated";
     if (first() !== "mutated") throw new Error("snapshot capture");
   });
   function joined(extra?: string[]): string { return fixtures.concat(["third"], extra ?? []).join("|"); }
   it("multi-array fixture helper", () => {
     if (joined(["fourth"]) !== fixtures.join("|") + "|third|fourth") throw new Error("wrong concat");
     if (joined() !== fixtures.join("|") + "|third") throw new Error("wrong default concat");
   });
 });
 describe("later", () => it("independent", () => {}));`
	// Imported aliases must be certified by binding identity, not spelling.
	source = strings.ReplaceAll(source, `describe("helpers"`, `suite("helpers"`)
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime) + "\nexport {describe, it};", "test/host.ts": `import {describe, describe as suite, it} from "../runtime/mocha";` + source})
	l, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, l)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed, Trapped int
		Results                 []struct{ Title string }
		Initialization          []any
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 7 || got.Failed != 0 || got.Trapped != 0 || len(got.Initialization) != 0 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
	js := `const titles=[]; const suites=[]; function describe(t,f){suites.push(t);try{f()}finally{suites.pop()}} function it(t,f){titles.push([...suites,t].join(" "));f()} const suite=describe;` + "\n" + source + "\nconsole.log(JSON.stringify(titles));"
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]))`, js).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	titles := []string{}
	for _, r := range got.Results {
		titles = append(titles, r.Title)
	}
	var nodeTitles []string
	if err := json.Unmarshal(want, &nodeTitles); err != nil {
		t.Fatal(err, string(want))
	}
	if !reflect.DeepEqual(titles, nodeTitles) {
		t.Fatalf("Go %q Node %q", titles, nodeTitles)
	}
}

func TestRegistrationFixtureProofBoundaries(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": `
 describe("capture-free", () => {
   function read(): string { return "no data"; }
   it("refused", () => { if (read() === "") throw new Error("empty"); });
 });
 describe("escaping", () => {
   const data = "fixture";
   function read(): string { return data; }
   it("refused", () => { const alias = read; if (alias() === "") throw new Error("empty"); });
 });
 describe("late write", () => {
   let data = "before";
   function read(): string { return data; }
   it("refused", () => { if (read() !== "after") throw new Error("stale"); });
   data = "after";
 });
 describe("late arrow", () => {
   it("refused", () => { if (read("value") !== "value") throw new Error("wrong"); });
   const read = (value: string): string => value;
 });
 describe("later", () => it("independent", () => {}));`})
	l, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, l)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed, Trapped int
		Initialization          []any
	}
	// Shared lexical bindings (main 37f425c) run the capture-free, escaping and
	// late-write fixtures with JS semantics; each body checks the JS value, so
	// passing is the correct outcome. Reading a const arrow before its
	// declaration still traps.
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 1 || got.Passed != 4 || got.Failed != 0 || got.Trapped != 1 || len(got.Initialization) != 0 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
}

func TestRegistrationHelperReceiverBoundary(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": `
 describe("receiver", () => {
   function own(): string { return String(this); }
   function via(): string { return own(); }
   it(via(), () => {});
 });
 describe("later", () => it("independent", () => {}));`})
	l, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	refused := false
	for _, d := range l.Diagnostics {
		refused = refused || strings.Contains(d.String(), "closure uses this outside an instance method")
	}
	if !refused {
		t.Fatal("receiver-dependent suite lost its existing closure refusal")
	}
	files, err := EmitAbaplintTests(dir, l)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed, Trapped int
		Initialization          []struct{ Status string }
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 1 || got.Failed != 0 || got.Trapped != 0 || len(got.Initialization) != 1 || got.Initialization[0].Status != "trapped" {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
}
