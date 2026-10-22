package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGoMathCeilNodeOracle(t *testing.T) {
	source := `class Probe {
 static count=0;
 static once(n:number):number {Probe.count++;return n;}
 static ceil(n:number):number {return Math.ceil(Probe.once(n));}
 static run():string {let out="";for(const n of [-1.5,-0.5,-0,0,0.5,1.5,4503599627370495.5]){const c=Probe.ceil(n);out+=c+";";}return out+Probe.count;}
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
	edgeMain := fmt.Sprintf(`v:=%s(-0.5);fmt.Println(1/v);fmt.Println(%s(ceilEdgeInput(0)));fmt.Println(%s(ceilEdgeInput(1)));fmt.Println(%s(ceilEdgeInput(-1)))`, goEntry("probe.ts.Probe", "ceil"), goEntry("probe.ts.Probe", "ceil"), goEntry("probe.ts.Probe", "ceil"), goEntry("probe.ts.Probe", "ceil"))
	if got := runGoHIR(t, p, edgeMain, func(files map[string]string) {
		files["runtime.go"] += `
func ceilEdgeInput(sign int) float64 {if sign==0 {return math.NaN()};return math.Inf(sign)}
`
	}); got != "-Inf\nNaN\n+Inf\n-Inf\n" {
		t.Fatal(got)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatalf("ABAP guarded ceil emission: %v", err)
	}
}

func TestMathCeilZeroAlignmentNodeAndABAP(t *testing.T) {
	source := `class Probe {
 static count=0;
 static once(n:number):number {Probe.count++;return n;}
 static ceil(n:number):number {return Math.ceil(Probe.once(n));}
 static refused():number {return Math.ceil(-0.5);}
 static layout(sizes:number[],aligns:number[]):number {
  let max=0,align=1;
  for(let i=0;i<sizes.length;i++) {
   align=Math.max(align,aligns[i]);
   max=Math.ceil(max/aligns[i])*aligns[i]+sizes[i];
  }
  return Math.ceil(max/align)*align;
 }
 static run():string {return [Probe.ceil(0),Probe.ceil(1.2),Probe.ceil(-1.2),Probe.layout([12,8],[2,8]),Probe.count].join("|");}
}`
	want := regexpOptionalCapturesNode(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "refused", "cx_sy_range_out_of_bounds")
}
func TestGoMathCeilShadowedBinding(t *testing.T) {
	source := `class LocalMath {ceil(x:number):number {return x+10;}} class Probe {static run():number {const Math=new LocalMath();return Math.ceil(2);}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "12\n" {
		t.Fatal(got)
	}
}
