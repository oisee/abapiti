package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestJSONStringifyFreshLiteralNodeOracle(t *testing.T) {
	source := `class Probe {static log = "";static tick(s: string): string {Probe.log += s;return s;}
 static run(): string {
  const alias: any = JSON.parse('{"unchanged":1}');
  const text = JSON.stringify({
   z: Probe.tick("z"), "10":10, "2":2, "01":1, gone:undefined,
   header:{description:Probe.tick("d"),originalLanguage:"en"},
   format:{dataType:"CHAR",length:1},
   alias:alias
  });
  alias["unchanged"] = 2;
  return text + "/" + Probe.log + "/" + JSON.stringify(alias);
 }
 static rows(): string { return JSON.stringify({rows:[{b:2,a:1},{missing:undefined,empty:""}]}); }
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
	// The original array-of-native-rows case is retained as a domain negative,
	// not silently serialized in guessed declaration order. Node supports it;
	// this lowering has not repaired its row-array storage ABI.
	rowWant, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.rows())")`, source).CombinedOutput()
	if err != nil || string(rowWant) != "{\"rows\":[{\"b\":2,\"a\":1},{\"empty\":\"\"}]}\n" {
		t.Fatal(err, string(rowWant))
	}
	main := fmt.Sprintf(`func(){defer func(){if r,ok:=recover().(trap);ok {fmt.Print(r.Error())} else {panic("native row order must refuse")}}();%s();panic("writer guessed native row order")}()`, goEntry("probe.ts.Probe", "rows"))
	if got := runGoHIR(t, p, main); !strings.Contains(got, "own-property order") {
		t.Fatal(got)
	}
	// The separate existing native-order negative still refuses; fresh literal
	// context must not change how named instances are serialized.
}
