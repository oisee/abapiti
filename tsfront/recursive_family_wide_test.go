package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// A wide union payload has no ABAP constructor. Its shared view must still
// preserve aliasing when an untyped record enters through from_dynamic.
func TestRecursiveFamilyWidePayloadABAPOracle(t *testing.T) {
	var fields, values []string
	for i := 0; i < 45; i++ {
		fields = append(fields, fmt.Sprintf("f%d:number", i))
		values = append(values, fmt.Sprintf(`"f%d":%d`, i, i))
	}
	source := fmt.Sprintf(`type Tree = {kind:'wide';child?:Tree;%s} | {kind:'leaf';value:number};
class Probe {
 static check(v:any):Tree { return v; }
 static run():string {
  const raw:any=JSON.parse('{"kind":"wide",%s}');
  const x=Probe.check(raw);
  if(x.kind==='wide') {
   const before=x.f44;
   raw.f44=99;
   const after=x.f44;
   const alias:any=x;
   alias.f0=7;
   return before+':'+after+':'+raw.f0+':'+x.f0;
  }
  return 'wrong';
 }
 static wrong():string {
  const x=Probe.check(JSON.parse('{"kind":"wide","f0":1}'));
  return x.kind;
 }
}`, strings.Join(fields, ";"), strings.Join(values, ","))
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	wide := false
	for _, u := range p.Unions {
		for _, a := range u.Alternatives {
			for _, c := range p.Classes {
				wide = wide || c.Name == a.Payload.Name && c.Ctor != nil && len(c.Ctor.Params) > 40
			}
		}
	}
	if !wide {
		t.Fatal("probe did not produce a wide union payload")
	}
	if got := runGoHIR(t, cloneProgram(t, p), fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "wrong", "cx_sy_range_out_of_bounds")
}
