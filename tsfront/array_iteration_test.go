package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// findLast scans from the end (index and array arguments included);
// entries() yields [index, value] rows for references, strings and numbers.
func TestArrayFindLastEntriesNodeOracle(t *testing.T) {
	source := `type Row = {name:string; n:number};
class Probe {
 static run():string {
  const rows:Row[] = [{name:'a',n:1},{name:'b',n:2},{name:'c',n:2}];
  const last = rows.findLast((r) => r.n === 2);
  const none = rows.findLast((r) => r.n > 5);
  const at = rows.findLast((r, i) => i < 2);
  const out:string[] = [];
  for (const [i, r] of rows.entries()) out.push(i + r.name);
  const words = ['x','y'];
  for (const [i, w] of words.entries()) out.push(w + i);
  const nums = [7, 8];
  for (const [i, v] of nums.entries()) out.push(String(i * 10 + v));
  const parts:string[] = [last ? last.name : '-', none ? none.name : '-', at ? at.name : '-', out.join(',')];
  return parts.join(' ');
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
}
