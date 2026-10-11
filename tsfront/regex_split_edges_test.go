package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGoRegExpSplitNonfiniteLimitNodeOracle(t *testing.T) {
	source := `class Probe {static run(limit:number):string { return 'a,b,c'.split(/,/,limit).join('|'); }}`
	want, err := exec.Command("node", "-e", `for(const limit of [NaN,Infinity,-Infinity,-0,4294967295,4294967298]) console.log('a,b,c'.split(/,/,limit).join('|'))`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, `for _,lim:=range splitEdgeLimits(){fmt.Println(`+goEntry("probe.ts.Probe", "run")+`(lim))}`, func(files map[string]string) {
		files["runtime.go"] += `
func splitEdgeLimits() []float64 {return []float64{math.NaN(),math.Inf(1),math.Inf(-1),math.Copysign(0,-1),4294967295,4294967298}}
`
	})
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestGoRegExpSplitLongInputNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static run():string {
 const s='x'.repeat(1101)+' ,a, ';
 return s.split(/\s+/).join('|')+'!'+s.split(/a*/).join('|')+'!'+s.split(/(,)/).join('|')+'!'+s.split(/\b/).join('|');
 }}`)
}

func TestGoRegExpSplitEvaluationOrderNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static out='';
 static receiver():string {Probe.out+='R';return ',a,';}
 static pattern():RegExp {Probe.out+='P';return /(,)/g;}
 static limit():number {Probe.out+='L';return 4;}
 static run():string {const a=Probe.receiver().split(Probe.pattern(),Probe.limit());return Probe.out+':'+a.join('|');}
 }`)
}

func TestGoRegExpSplitReviewedRepeatedCapturesRefuse(t *testing.T) {
	pattern := `^!?((\w*\/\w+\/)|(\w*\/\w+\/)*[\w\*$%#]+)$`
	source := `class Probe {static run(pattern:string, limit:number):string { return 'x'.split(new RegExp(pattern),limit).join('|'); }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run(process.argv[2],0));console.log(Probe.run(process.argv[2],20))")`, source, pattern).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	entry := goEntry("probe.ts.Probe", "run")
	main := fmt.Sprintf(`fmt.Println(%s(str(%q),0));func(){defer func(){if _,ok:=recover().(trap);!ok{panic("missing capture-safe grammar trap")};fmt.Println("refused repeated captures")}();%s(str(%q),20);panic("accepted")}()`, entry, pattern, entry, pattern)
	if got := runGoHIR(t, p, main); got != "\nrefused repeated captures\n" {
		t.Fatal(got)
	}
	if !strings.HasPrefix(string(want), "\n") {
		t.Fatal(string(want))
	}
}

func TestGoRegExpSplitUndefinedCaptureABIRefuses(t *testing.T) {
	// The string[] ABI cannot carry an absent capture through every array
	// operation. Refuse at split, before losing undefined in slice/iteration.
	source := `class Probe {static run():string { return ','.split(/(,)(x)?/)[2]; }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(typeof Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if strings.TrimSpace(string(want)) != "undefined" {
		t.Fatal(string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	main := fmt.Sprintf(`func(){defer func(){v:=recover();if _,ok:=v.(trap);!ok{panic(fmt.Sprintf("expected typed trap, got %%v",v))};fmt.Println("refused undefined capture")}();%s();panic("accepted")}()`, goEntry("probe.ts.Probe", "run"))
	got := runGoHIR(t, p, main)
	if got != "refused undefined capture\n" {
		t.Fatal(got)
	}
}
