package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestObjectEntriesRecordGoMatchesNode(t *testing.T) {
	source := `export class Item { name: string; constructor(name: string) { this.name = name; } }
export class Probe {
 static calls: number = 0;
 static make(): {[key: string]: Item} {
  Probe.calls += 1;
  const record: {[key: string]: Item} = {};
  for (const key of ["b", "10", "2", "01", "", "4294967294", "4294967295", "0", "a", "100", "-1", "000", "😀", " 1", "1.0", "99999999999"]) {
   record[key] = new Item(key);
  }
  return record;
 }
 static run(): string {
  const record = Probe.make();
  const shared = new Item("old");
  record["2"] = shared;
  const entries = Object.entries(record);
  record["b"] = new Item("replacement");
  shared.name = "shared";
  record["new"] = new Item("new");
  let result = "";
  for (const [key, value] of entries) { result += "[" + key + ":" + value.name + "]"; }
  const empty: {[key: string]: string} = {};
  return result + "/" + entries.length + "/" + Probe.calls + "/" + Object.entries(empty).length;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	dir := t.TempDir()
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", oracle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
}

func TestObjectEntriesRefusesOtherObjectModels(t *testing.T) {
	for _, source := range []string{
		`export class Probe { run(value: {[key: string]: string | undefined}): number { return Object.entries(value).length; } }`,
		`export class Probe { run(): number { const map = new Map<string,string>(); map.set("a","x"); return Object.entries(map).length; } }`,
		`export class Probe { run(): number { return Object.entries("text").length; } }`,
	} {
		requireDiagnostic(t, sourceProbe(t, source), "unsupported-call")
	}
}

// The rule has a local push helper in both runTextPool and runDomain. Once
// entries allows runTextPool to lower, both helpers must coexist.
func TestObjectEntriesRepeatedCallbackNames(t *testing.T) {
	source := `export class Probe {
  first(): string { const values: string[] = []; const push = (value: string) => { values.push(value); }; push("one"); return values.join(""); }
  second(): string { const values: string[] = []; const push = (value: string) => { values.push(value); }; push("two"); return values.join(""); }
  static run(): string { const probe = new Probe(); return probe.first() + "/" + probe.second(); }
 }`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())"); got != "one/two" {
		t.Fatal(got)
	}
}

// Native declaration-order layouts retain their existing proof; class views use
// backing-object metadata. Other native structural layouts still refuse.
func TestObjectEntriesNominalNodeOracle(t *testing.T) {
	source := `class Item {value="x";} class Probe {static run():number {return Object.entries(new Item()).length;}}`
	p := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if got := runGoHIR(t, p, "fmt.Println("+goEntry("input.ts.Probe", "run")+"())"); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
