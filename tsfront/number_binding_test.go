package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/golang"
)

// Grouped labels lower one source block multiple times. Those declarations
// share a checker symbol, while a shadowing declaration has a different one.
func TestNumberBindingsRepeatedSwitchBody(t *testing.T) {
	for _, outer := range []string{"const outer=3;", "const value=3;"} {
		for _, declaration := range []string{"const value=7;", "let value=7;", "const [value]=[7];"} {
			t.Run(outer+"/"+declaration, func(t *testing.T) {
				p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
export class Probe {
 run(label:string):number {
  ` + outer + `
  switch(label) {
   case "a":
   case "b":
   case "c": {` + declaration + ` return value;}
   default: return 3;
  }
 }
}`}, []string{"probe.ts"})
				if _, err := emitABAP(t, p); err != nil {
					t.Fatal(err)
				}
				if _, err := golang.Emit(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestNumberBindingsGroupedLabelsMatchNode(t *testing.T) {
	source := `export class Probe {
 static pick(label:string):number {
  const value=3;
  switch(label) {
   case "a": case "b": case "c": {
    let value=7;
    {const value=11; if(label==="b") {return value;}}
    value=value+1;
    return value;
   }
   default: return value;
  }
 }
 static run():string {
  return ""+Probe.pick("a")+","+Probe.pick("b")+","+Probe.pick("c")+","+Probe.pick("z");
 }
}`
	p := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	oracle := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", oracle).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
}
