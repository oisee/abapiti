package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestNumberDivisionZeroNodeAndABAP(t *testing.T) {
	source := `class Probe {
 static divide(a:number,b:number):number {return a/b;}
 static zeroZero():number {return Probe.divide(0,0);}
 static positiveZero():number {return Probe.divide(1,0);}
 static negativeZero():number {return Probe.divide(-1,0);}
 static run():string {return String(Probe.divide(6,3));}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	want := regexpOptionalCapturesNode(t, source)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	for _, tc := range []struct{ method, want string }{{"zeroZero", "NaN"}, {"positiveZero", "Infinity"}, {"negativeZero", "-Infinity"}} {
		t.Run(tc.method, func(t *testing.T) {
			out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(String(Probe."+process.argv[2]+"()))")`, source, tc.method).CombinedOutput()
			if err != nil || string(out) != tc.want+"\n" {
				t.Fatalf("Node: %v %s", err, out)
			}
			// ABAP f cannot carry JS nonfinite values. All three must refuse before
			// ABAP can turn 0/0 into zero or raise its native division exception.
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), tc.method, "cx_sy_range_out_of_bounds")
		})
	}
}
