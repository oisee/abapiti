package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// Collections built from iterables and spread from Map/Set views, the
// ordinary typed-TS idioms gogen and abaplint-style code use.
func TestMapSetIterablesNodeOracle(t *testing.T) {
	source := `class Item {name: string; size: number; constructor(name: string, size: number) {this.name = name; this.size = size;}}
class Probe {
 static run(): string {
  const items = [new Item("b", 2), new Item("a", 1), new Item("b", 3)];
  const byName = new Map(items.map((i) => [i.name, i] as [string, Item]));
  const pairs: [string, number][] = [["x", 1], ["y", 2], ["x", 3]];
  const counts = new Map<string, number>(pairs);
  const names = new Set(items.map((i) => i.name));
  const keys = [...counts.keys()];
  const values = [...counts.values()];
  const both = new Set([...names, ...counts.keys()]);
  const copy = new Map(counts);
  copy.set("z", 9);
  const fromKeys = new Set(counts.keys());
  return byName.size + ":" + byName.get("b")!.size + "|" + counts.get("x") + ":" + keys.join(",") + ":" + values.join(",") +
   "|" + [...names].join(",") + "|" + [...both].join(",") + "|" + counts.size + "/" + copy.size + "|" + [...fromKeys].join(",");
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
