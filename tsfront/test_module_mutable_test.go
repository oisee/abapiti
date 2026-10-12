package tsfront

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestMutableModuleInitializationNodeOracle(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := `class Registry { value: string; constructor(value: string) { this.value = value; } }
 let reg: Registry = new Registry("initial");
 let hookReg: Registry | undefined;
 function replace(value: string): string { reg = new Registry(value); return reg.value; }
 describe("module", () => {
  before(() => { if (reg.value !== "initial") throw new Error("init did not precede hook"); hookReg = new Registry("before"); });
  beforeEach(() => { reg = new Registry("each"); });
  it("first", () => { if (reg.value !== "each" || hookReg?.value !== "before" || replace("first") !== "first") throw new Error("wrong first"); });
  it("second", () => { if (reg.value !== "each" || replace("second") !== "second") throw new Error("wrong second"); });
 });`
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": source})
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
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 2 || got.Failed != 0 || got.Trapped != 0 || len(got.Initialization) != 0 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
	// Independent Node execution: register bodies, then run hooks and bodies,
	// rather than invoking it immediately during suite registration.
	js := `const titles=[], tests=[], all=[], each=[]; let suite="";
 function describe(t,f){suite=t;f()} function it(t,f){tests.push([suite+" "+t,f])}
 function before(f){all.push(f)} function beforeEach(f){each.push(f)}
 ` + source + `for(const f of all)f();for(const [t,f] of tests){for(const h of each)h();f();titles.push(t)}console.log(JSON.stringify(titles));`
	out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]))`, js).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var want []string
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err, string(out))
	}
	var titles []string
	for _, r := range got.Results {
		titles = append(titles, r.Title)
	}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("Go %q Node %q", titles, want)
	}
}

func TestMutableModuleFailedInitializerStaysGuarded(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": `
 class Value { value = "not fabricated"; }
 function broken(): Value { return new Date() as any; }
 let bad: Value = broken();
 let primitive: number;
 describe("recovery", () => {
  it("guarded", () => { if (bad.value === "") throw new Error("fabricated"); });
  it("primitive refused", () => { if (primitive === 0) throw new Error("fabricated zero"); });
  it("independent", () => {});
 });`})
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
		Initialization          []struct{ Status string }
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 1 || got.Passed != 1 || got.Failed != 0 || got.Trapped != 2 || len(got.Initialization) != 2 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
	for _, r := range got.Initialization {
		if r.Status != "trapped" {
			t.Fatal(r)
		}
	}
}
