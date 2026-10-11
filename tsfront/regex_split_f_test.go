package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

// F's original cases on the single G splitter. Node, not F's narrower
// implementation domain, decides capture/zero-width/empty results.
func TestGoRegExpSplitFNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const patterns:RegExp[]=[/\s+/,/[\s.,:(]+/,/x/i,/x/g,/x/y];
 const inputs:string[]=['','a b',' a b ','a,b:(c','\t\n','a\u00a0b\ufeffc','a\u180eb','😀 x 😀','x','xx','aXb','one\u2028two'];
 let out='';for(const re of patterns){for(const input of inputs){re.lastIndex=2;const parts=input.split(re);out+=parts.length+':'+parts.join('|')+':'+re.lastIndex+';';}}
 return out;
 }
 static capture():string[] {return 'a b'.split(/(\s+)/);}
 static boundary():string[] {return 'ab'.split(/\b/);}
 static empty():string[] {return ''.split(/a*/);}
 static results():string {return JSON.stringify(Probe.capture())+'\n'+JSON.stringify(Probe.boundary())+'\n'+JSON.stringify(Probe.empty());}
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run());for(const method of ['capture','boundary','empty'])console.log(JSON.stringify(Probe[method]()))")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	// Check the original F source, without the newly added Go JSON serializer
	// masking the split-specific ABAP capability refusal.
	abapSource := source[:strings.Index(source, " static results()")] + "}"
	abapProgram := lowerStatementsProbe(t, map[string]string{"probe.ts": abapSource}, []string{"probe.ts"})
	if _, err = abap.Emit(abapProgram); err == nil || !strings.Contains(err.Error(), "string.splitRegex") {
		t.Fatalf("missing ABAP refusal: %v", err)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	main := fmt.Sprintf("fmt.Println(%s());fmt.Println(%s());", goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "results"))
	got := runGoHIR(t, p, main)
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
