package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// Regression for syntax.ts's inferred character variable: object rest must
// remove derivedFromConstant, even when nominal storage reuses the source
// shape because every excluded property is optional.
func TestGoObjectRestExcludesOptionalPropertiesNodeOracle(t *testing.T) {
	source := `type Data = {name?: string, derivedFromConstant?: boolean};
 class Probe {static run(): string {
 const data: Data = {name: "kept", derivedFromConstant: true};
 const {derivedFromConstant, ...rest} = data;
 rest.name = "changed";
 const {name: renamed, ...other} = data;
 return String((rest as Data).derivedFromConstant) + ":" + rest.name + ":" + data.name + ":" + derivedFromConstant + ":" + renamed + ":" + String((other as Data).name) + ":" + other.derivedFromConstant;
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
