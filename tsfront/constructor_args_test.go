package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGoDynamicConstructorArgsNodeOracle(t *testing.T) {
	source := `class A {name: string; suffix: string; constructor(name: string, suffix="!") {this.name=name;this.suffix=suffix;} value():string {return this.name+this.suffix;}}
 class B extends A {}
 class Probe {static run(): string {const constructors: any[]=[A,B];let log="";const c=constructors[1];const o=new c("live");const p=new c("two","?","ignored");return o.value()+":"+p.value();}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	// Read methods through a checked structural interface instead of arbitrary
	// dynamic callable properties (which remain a separate refusal).
	source = `interface Value {value():string;}` + source
	source = strings.ReplaceAll(strings.ReplaceAll(source, "const o=new c", "const o:Value=new c"), "const p=new c", "const p:Value=new c")
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
