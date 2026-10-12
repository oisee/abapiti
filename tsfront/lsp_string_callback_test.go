package tsfront

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A string field whose declaration happens to be in vscode-languageserver-
// types must not be misclassified as an external namespace factory.
func TestLSPStringFieldSplitNodeOracle(t *testing.T) {
	prog, ds := lspProbe(t, `import {TextEdit, Range} from "vscode-languageserver-types";
 export class Probe { static run(): string {
  const edit = TextEdit.replace(Range.create(0, 0, 0, 0), "a|b||c");
  return edit.newText.split("|").join("/");
 }}`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if es := hir.Verify(prog); len(es) != 0 {
		t.Fatal(es)
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("main.ts.Probe", "run")+"().String())")
	want, err := exec.Command("node", "-p", `({newText:"a|b||c"}).newText.split("|").join("/")`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
