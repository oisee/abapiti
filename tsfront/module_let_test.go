package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// Mutable module variables (module-level let). The critic-r1 super_reorder
// hazard: a derived constructor writes the module variable before super(),
// and a base field initializer reads it; both backends keep that order.
func TestModuleLetNodeOracle(t *testing.T) {
	source := `let count: number = 0;
let names: string[] = [];
export class Base { public trace: number = count; public constructor() {} public run(): number { return this.trace; } }
export class Probe extends Base {
 public constructor() { count = count + 1; names.push("p" + count); super(); }
 static go(): string { const a = new Probe(); const b = new Probe(); names = names.concat(["end"]); return a.run() + ":" + b.run() + ":" + count + ":" + names.join(","); }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]).replace(/export /g, "")+"\nconsole.log(Probe.go())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "go")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
