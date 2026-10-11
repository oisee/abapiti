package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestRulesReferenceOrUndefinedGoMatchesNode(t *testing.T) {
	source := `export interface Named { getName(): string; }
export class Item implements Named { name: string; constructor(name: string) { this.name = name; } getName(): string { return this.name; } }
export class Probe {
 static calls: number = 0;
 static lookup(present: boolean): Item | undefined { Probe.calls += 1; return present ? new Item("ok") : undefined; }
 static name(present: boolean): string {
  const found = Probe.lookup(present) as Item || undefined;
  return found ? found.getName() : "absent";
 }
 static view(present: boolean): string {
  const found = Probe.lookup(present) as Named || undefined;
  return found ? found.getName() : "absent";
 }
 static run(): string { return Probe.name(true) + "/" + Probe.name(false) + "/" + Probe.view(true) + "/" + Probe.view(false) + "/" + Probe.calls; }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	dir := t.TempDir()
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", oracle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
}
