package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestObservedScalarArrayIndexNodeOracle(t *testing.T) {
	const source = `class Probe {
 static calls=0;
 static words():string[] {Probe.calls++;return ["METHOD"];}
 static index():number {Probe.calls++;return -1;}
 static required():string {const words=["METHOD"];return words[6];}
 static run():string {
 const words=["METHOD"],parts="a-b".split("-");const numbers=[0],flags=[false];
 let ok=words[words.length-2]!=="ENDEXEC"&&words[-1]!==""&&words[0]==="METHOD";
 if(parts[2]||words[-1]){ok=false;}
 ok=ok&&parts[2]===undefined&&numbers[-1]!==0&&flags[-1]!==false;
 ok=ok&&Probe.words()[Probe.index()]===undefined;
 return ok+":"+Probe.calls;
 }
 }`
	want := conversionOracle(t, source)
	if want != "true:2\n" {
		t.Fatalf("Node: %q", want)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	// Do not turn every scalar read into an optional ABI. A required return
	// outside these observation contexts must still use the checked IndexGet.
	required := false
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if m.Name == "required" {
				viewWalk(m.Body, nil, func(x, _ *hir.Expr) { required = required || x.Kind == hir.IndexGet })
			}
		}
	}
	if !required {
		t.Fatal("required scalar read lost its checked boundary")
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}
