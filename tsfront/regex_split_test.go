package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestGoRegExpSplitNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const patterns:RegExp[]=[/\s+/,/[\s.,:(]+/,/x/i,/x/g,/x/y];
 const inputs:string[]=['','a b',' a b ','a,b:(c','\t\n','a\u00a0b\ufeffc','a\u180eb','😀 x 😀','x','xx','aXb','one\u2028two'];
 let out='';for(const re of patterns){for(const input of inputs){re.lastIndex=2;const parts=input.split(re);out+=parts.length+':'+parts.join('|')+':'+re.lastIndex+';';}}
 return out;
 }
 static capture():string[] {return 'a b'.split(/(\s+)/);}
 static boundary():string[] {return 'ab'.split(/\b/);}
 static empty():string[] {return ''.split(/a*/);}
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err = abap.Emit(p); err == nil || !strings.Contains(err.Error(), "string.splitRegex") {
		t.Fatalf("missing ABAP refusal: %v", err)
	}
	main := fmt.Sprintf("fmt.Println(%s());", goEntry("probe.ts.Probe", "run"))
	for _, method := range []string{"capture", "boundary", "empty"} {
		main += fmt.Sprintf(`func(){defer func(){if _,ok:=recover().(trap);!ok{panic("missing split delimiter refusal")}}();%s()}();`, goEntry("probe.ts.Probe", method))
	}
	got := runGoHIR(t, p, main)
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
