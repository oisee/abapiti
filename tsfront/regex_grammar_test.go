package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoRegExpWordClassesGroupsNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const patterns:RegExp[]=[/^@\w+$/,/^[\w_]+$/,/^\d+E$/i,/^\W+$/,/^\D+$/,/^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+$/i,/^"(?:[^"]|"")*"$/, /\.fugr\.(#\w+#)?sapl/i, /@EndUserText\.label: '([\w,.:-=#%&() ]+)'/, /^(a|bc)(d)?$/];
 const inputs:string[]=["@AB_09","@é","@K","@ſ","@😀","@abc\n","@abc\r","@abc\u2028","@abc\u2029","a_09","09E","09e","!😀", "foo_bar", "FOO_BAR", '"a""b"', "z.fugr.sapl", "z.FUGR.#NS#sapl", "@EndUserText.label: 'foo=bar - ok'", "a", "ad", "bc", "bcd"];
 let out="";for(const p of patterns){for(const s of inputs){const hit=p.exec(s);out+=hit===null?"none;":hit[0]+":"+hit.index+":"+String(hit[1])+":"+String(hit[2])+";";}}
 return out;
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
