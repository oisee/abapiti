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
class Bag {
 static made = 0;
 keys(): string[] { return ["k1", "k2"]; }
 static make(): Bag { Bag.made++; return new Bag(); }
}
class Probe {
 static more(): string {
  const own = [...Bag.make().keys()];
  const opt = new Map<string, number | undefined>([["a", undefined], ["b", 2]]);
  const optCopy = new Map<string, number | undefined>(opt);
  const re = new Map<string, number>([["p", 1], ["q", 2], ["r", 3]]);
  re.delete("p");
  re.set("p", 4);
  const reCopy = new Map(re);
  const s1 = new Set(["x", "y"]);
  const s2 = new Set(["y", "z"]);
  const dup = new Set([...s1, ...s2, ...s1]);
  const none = new Map<string, number>([]);
  const noneCopy = new Map(none);
  const noneSet = new Set([...new Set<string>()]);
  return own.join(",") + ":" + Bag.made + "|" + optCopy.size + ":" + optCopy.has("a") + ":" + (optCopy.get("a") === undefined) + ":" + optCopy.get("b") +
   "|" + [...reCopy.keys()].join(",") + "=" + [...reCopy.values()].join(",") + "|" + [...dup].join(",") + "|" + noneCopy.size + ":" + noneSet.size;
 }
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
   "|" + [...names].join(",") + "|" + [...both].join(",") + "|" + counts.size + "/" + copy.size + "|" + [...fromKeys].join(",") + "\n" + Probe.more();
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
