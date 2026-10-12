package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRecursionNodeOracle(t *testing.T) {
	source := `export class Probe {
 static run(): string {
  const values: number[] = [];
  const visit = (n: number): void => { values.push(n); if (n > 0) { visit(n - 1); } };
  visit(3);
  const sum = (n: number): number => { if (n === 0) { return 0; } return n + sum(n - 1); };
  return values.join(",") + "/" + sum(4);
 }
 static other(): string {
  const visit = (n: number): void => { if (n > 0) { visit(n - 1); } };
  visit(2); return "done";
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String()+\"/\"+"+goEntry("input.ts.Probe", "other")+"().String())")
	dir := t.TempDir()
	path := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run()+\"/\"+Probe.other());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSpace(string(out)) {
		t.Fatalf("Go %q Node %q", got, out)
	}
}
