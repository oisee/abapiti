package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuralUnionConstructorRefusalNodeOracle(t *testing.T) {
	source := `interface A {a: string;}
interface B {b: string;}
export class Probe {
 static name(x: A|B): string {return x.constructor.name;}
 static run(): string {const x: A={a:"a"};return Probe.name(x);}
}`
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "Object" {
		t.Fatal(err, string(out))
	}
	p := sourceProbe(t, source)
	_, diags := lowerProbe(t, p)
	refused := false
	for _, d := range diags {
		refused = refused || d.Category == "unsupported-expr" && strings.Contains(d.Message, "union field constructor")
	}
	if !refused {
		prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
		got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
		t.Fatalf("structural union was not refused: Node Object, Go %q", got)
	}
}
