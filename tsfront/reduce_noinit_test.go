package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// arr.reduce(cb) without an initial value: the first present element starts
// the accumulator, holes are skipped (index argument included), and an empty
// or all-holes array throws a TypeError.
func TestReduceNoInitialNodeOracle(t *testing.T) {
	source := `class Probe {
 static join(parts:string[]):string {return parts.reduce((a, b) => a+'+'+b);}
 static idx(parts:string[]):string {return parts.reduce((a, b, i) => a+b+i);}
 static run():string {
  const sparse:string[] = [];
  sparse[2] = 'x'; sparse[4] = 'y';
  let empty = 'none';
  try { Probe.join([]); } catch (e) { empty = e instanceof TypeError ? 'TypeError' : 'other'; }
  let holes = 'none';
  const allHoles:string[] = []; allHoles[3] = 'z'; delete allHoles[3];
  try { Probe.join(allHoles); } catch (e) { holes = e instanceof TypeError ? 'TypeError' : 'other'; }
  return [Probe.join(['a']), Probe.join(['a','b','c']), Probe.idx(['a','b','c']), Probe.join(sparse), Probe.idx(sparse), empty, holes].join(' ');
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
