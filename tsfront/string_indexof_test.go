package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStringIndexOfFromGoMatchesNode(t *testing.T) {
	source := `export class Probe {
 static order: string = "";
 static receiver(): string { Probe.order += "R"; return "abca"; }
 static needle(): string { Probe.order += "N"; return "a"; }
 static position(): number { Probe.order += "P"; return 1.9; }
 static at(position?: number): number { return "abca".indexOf("a", position); }
 static run(): string {
  const observed = Probe.receiver().indexOf(Probe.needle(), Probe.position());
  const results = [
   "abca".indexOf("a", 0), "abca".indexOf("a", 1), "abca".indexOf("a", 3),
   "abca".indexOf("a", 4), "abca".indexOf("a", 200), "abca".indexOf("a", -2),
   "abca".indexOf("x", 1), "abca".indexOf("", 1), "abca".indexOf("", 4),
   "abca".indexOf("", 200), "abca".indexOf("", -2), "".indexOf("", 1),
   "abca".indexOf("a", 1.9), "abca".indexOf("a", -1.9),
   "abca".indexOf("", 1e100), "abca".indexOf("a", -1e100),
   "abca".indexOf("a", undefined), Probe.at(), Probe.at(1),
   "😀x😀".indexOf("😀", 1), "😀x".indexOf("😀".charAt(1), 1),
   "\tx\t".indexOf("\t", 1)
  ];
  return results.join(",") + "/" + observed + "/" + Probe.order;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	if _, err := emitABAP(t, prog); err != nil {
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

func TestStringIndexOfFromRefusesCoercion(t *testing.T) {
	for _, source := range []string{
		`export class Probe { run(position: any): number { return "text".indexOf("e", position); } }`,
		`export class Probe { run(search: number): number { return "text".indexOf(search, 0); } }`,
	} {
		requireDiagnostic(t, sourceProbe(t, source), "unsupported-call")
	}
}
