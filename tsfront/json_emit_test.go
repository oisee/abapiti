package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestGoJSONDataGraphNodeOracle(t *testing.T) {
	source := `class Probe {static run(): string {
 const d: any = JSON.parse('{"z":"x","10":10,"2":2,"01":1}');
 d["gone"] = undefined;
 const a: number[] = [1,2,3]; delete a[1]; a[4]=5;
 d["a"] = a;
 const text = JSON.stringify(d);
 const copy: any = JSON.parse(text);
 d["z"] = "changed";
 return text + "/" + JSON.stringify(copy) + "/" + JSON.stringify(JSON.parse('"\\ud800😀\\n"'));
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	writer := lowerStatementsProbe(t, map[string]string{"writer.ts": `class Probe {static run(): string {return JSON.stringify(42);}}`}, []string{"writer.ts"})
	if _, err := abap.Emit(writer); err == nil || !strings.Contains(err.Error(), "json.stringifySubset") {
		t.Fatalf("ABAP must refuse JSON writer: %v", err)
	}
}

func TestGoJSONUnprovenNativeOrderRefuses(t *testing.T) {
	source := `class Native {b: number=1;a: number=2;} class Probe {static run(): string {return JSON.stringify(new Native());}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	main := fmt.Sprintf(`func(){defer func(){if _,ok:=recover().(trap);!ok {panic("native order must refuse")}}();%s();panic("writer guessed order")}();fmt.Println("ok")`, goEntry("probe.ts.Probe", "run"))
	if got := runGoHIR(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
