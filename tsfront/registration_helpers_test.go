package tsfront

import (
	"encoding/json"
	"os"
	"os/exec"
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
   const fixtures = [table("one", [field("A"), field("B")]), table("two", [field("C")])];
   fixtures.forEach(value => it(value, () => { if (value.length < 1) throw new Error("empty fixture"); }));
 });
 describe("later", () => it("independent", () => {}));`
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
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 3 || got.Failed != 0 || got.Trapped != 0 || len(got.Initialization) != 0 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
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
