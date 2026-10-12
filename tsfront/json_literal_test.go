package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestJSONStringifyFreshLiteralNodeOracle(t *testing.T) {
	source := `class Probe {static log = "";static tick(s: string): string {Probe.log += s;return s;}
 static run(): string {
  const alias: any = JSON.parse('{"unchanged":1}');
  const text = JSON.stringify({
   z: Probe.tick("z"), "10":10, "2":2, "01":1, gone:undefined,
   header:{description:Probe.tick("d"),originalLanguage:"en"},
   format:{dataType:"CHAR",length:1},
   alias:alias
  });
  alias["unchanged"] = 2;
  return text + "/" + Probe.log + "/" + JSON.stringify(alias);
 }
 static rows(): string { return JSON.stringify({rows:[{b:2,a:1},{missing:undefined,empty:""}]}); }
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
	// Retain the original row-array fixture and its exact Node value. The
	// writer now retains each literal's per-instance own-key metadata, not
	// guessed layout order; this is a strict value assertion, not a no-op.
	rowWant, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.rows())")`, source).CombinedOutput()
	if err != nil || string(rowWant) != "{\"rows\":[{\"b\":2,\"a\":1},{\"empty\":\"\"}]}\n" {
		t.Fatal(err, string(rowWant))
	}
	main := fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "rows"))
	if got := runGoHIR(t, p, main); got != string(rowWant) {
		t.Fatalf("Go %q Node %q", got, rowWant)
	}
}
