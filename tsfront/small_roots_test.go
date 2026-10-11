package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestGoObjectRootConstructor(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 class Item {}
 class Other {}
 class Probe {
 static inspect(value: object): string { return value.constructor.name; }
 static optional(value?: object): string { const ctor = value?.constructor?.name; return ctor === undefined ? "absent" : ctor; }
 static run(): string { return Probe.inspect(new Item()) + ":" + Probe.inspect(new Other()) + ":" + Probe.optional(new Item()) + ":" + Probe.optional(); }
 }`}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "Item:Other:Item:absent\n" {
		t.Fatal(got)
	}
}

// Keep the upstream method verbatim: the production source fingerprint must
// validate this proof, and a membership mutation must invalidate it.
const catchCommonSuperclassMethod = `private findCommonSuperclass(classNames: readonly string[], input: SyntaxInput): IClassDefinition | undefined {
    const lineages: IClassDefinition[][] = [];
    for (const className of classNames) {
      const lineage: IClassDefinition[] = [];
      let current = input.scope.findClassDefinition(className);
      const visited = new Set<string>();
      while (current && visited.has(current.getName().toUpperCase()) === false) {
        lineage.push(current);
        visited.add(current.getName().toUpperCase());
        current = input.scope.findClassDefinition(current.getSuperClass());
      }
      lineages.push(lineage);
    }

    return lineages[0]?.find(candidate => lineages.every(lineage =>
      lineage.some(item => item.getName().toUpperCase() === candidate.getName().toUpperCase())));
  }`

func TestGoCatchCommonSuperclassNodeOracle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("Node is required for the superclass oracle")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	source := `
 interface IClassDefinition { getName(): string; getSuperClass(): string; }
 class Def implements IClassDefinition {
 private name: string; private parent: string;
 constructor(name: string, parent: string) { this.name = name; this.parent = parent; }
 getName(): string { return this.name; } getSuperClass(): string { return this.parent; }
 }
 class Scope {
 private defs: Map<string, IClassDefinition> = new Map<string, IClassDefinition>();
 add(name: string, parent: string): void { this.defs.set(name, new Def(name, parent)); }
 findClassDefinition(name: string): IClassDefinition | undefined { return this.defs.get(name); }
 }
 class SyntaxInput { scope: Scope; constructor(scope: Scope) { this.scope = scope; } }
 class Catch {
 public choose(names: readonly string[], input: SyntaxInput): IClassDefinition | undefined { return this.findCommonSuperclass(names, input); }
 ` + catchCommonSuperclassMethod + `
 }
 class Probe {
 static one(scope: Scope, names: string[]): string { return new Catch().choose(names, new SyntaxInput(scope))?.getName() || "none"; }
 static run(): string {
 const scope = new Scope();
 scope.add("ROOT", ""); scope.add("A", "ROOT"); scope.add("B", "ROOT"); scope.add("Z", ""); scope.add("X", "Y"); scope.add("Y", "X");
 return [Probe.one(scope, ["A", "B"]), Probe.one(scope, ["A", "A"]), Probe.one(scope, ["ROOT", "A"]), Probe.one(scope, []), Probe.one(scope, ["missing", "A"]), Probe.one(scope, ["A", "Z"]), Probe.one(scope, ["X", "Y"])].join("|");
 }
 }
 `
	registry, err := RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	var entry overrides.Entry
	for _, e := range registry.Inventory() {
		if e.ID == "abaplint-dense-callback-Catch.findCommonSuperclass" {
			entry = e
		}
	}
	if entry.ID == "" {
		t.Fatal("missing production density proof")
	}
	if _, ok, err := registry.Lookup(entry.Key, catchCommonSuperclassMethod, "catch.ts"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	changed := strings.Replace(catchCommonSuperclassMethod, "lineage.some(item =>", "lineage.reverse().some(item =>", 1)
	if _, _, err := registry.Lookup(entry.Key, changed, "catch.ts"); err == nil {
		t.Fatal("changed callback accepted by density proof")
	}
	entry.Key.File = "probe.ts"
	fixture, err := overrides.New(entry)
	if err != nil {
		t.Fatal(err)
	}
	p := integerProbe(t, source)
	// Unpinned every now uses presence-sensitive, length-snapshot iteration.
	unpinned, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if got := runGoHIR(t, unpinned, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "ROOT|A|ROOT|none|none|none|X\n" {
		t.Fatal(got)
	}
	prog, ds, err := p.LowerWithReachability([]string{"probe.ts"}, fixture, nil)
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `const source = process.argv[1]; eval(require("node:module").stripTypeScriptTypes(source) + "\nprocess.stdout.write(Probe.run() + '\\n')");`, source)
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if string(want) != "ROOT|A|ROOT|none|none|none|X\n" {
		t.Fatal("unexpected Node oracle", string(want))
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
