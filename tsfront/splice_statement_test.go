package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// A splice whose result is discarded keeps the same mutation (the Go backend
// skips building the removed-elements array), including holes and a result
// that is used.
func TestSpliceStatementNodeOracle(t *testing.T) {
	source := `class Probe {
 static show(xs: (number | undefined)[]): string {
  let out = "";
  for (let i = 0; i < xs.length; i++) { const v = xs[i]; out += (v === undefined ? "u" : String(v)) + ","; }
  return out;
 }
 static run(): string {
  const a = [1, 2, 3, 4, 5];
  a.splice(1, 2);
  const b = ["x", "y", "z"];
  b.splice(-1, 5);
  b.splice(0, 0);
  const holes: (number | undefined)[] = [10, 20, 30, 40];
  delete holes[0];
  holes.splice(1, 1);
  const used = [7, 8, 9].splice(1, 1);
  const s = [1, 2, 3, 4];
  const tail = s.splice(1);
  s.push(9);
  tail.push(8);
  tail[0] = 7;
  const z = [5, 6];
  const all = z.splice(0);
  z.push(1);
  all.push(2);
  return a.join(",") + "|" + b.join(",") + "|" + holes.length + ":" + Probe.show(holes) + "|" + used.join(",") + "|" + s.join(",") + "/" + tail.join(",") + "|" + z.join(",") + "/" + all.join(",");
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
}
