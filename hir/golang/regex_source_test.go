package golang

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// RegExp.prototype.source escapes "/" only outside character classes (V8);
// abaplint prints it in naming messages, e.g. /^[A-Z_-\d/<> ]+$/i.
func TestRegExpSourceNodeOracle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	patterns := []string{"", "a/b", `a\/b`, "[/]", `[\]/]`, `^[A-Z_-\d\/<> ]+$`, `^[A-Z_-\d/<> ]+$`, "x[a/b]/y", "a\nb", "a\rb", "a b", `\[/`}
	data, _ := json.Marshal(patterns)
	want, err := exec.Command("node", "-e", `for(const p of JSON.parse(process.argv[1]))process.stdout.write(new RegExp(p).source+"\n")`, string(data)).Output()
	if err != nil {
		t.Fatal(err)
	}
	var main strings.Builder
	main.WriteString("for _,p:=range []string{")
	for _, p := range patterns {
		fmt.Fprintf(&main, "%q,", p)
	}
	main.WriteString(`}{fmt.Print(jsRegExpSource(p).String()+"\n")}`)
	if got := execute(t, &hir.Program{}, main.String()); got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
}
