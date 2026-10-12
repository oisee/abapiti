package tsfront

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrimitiveLastIndexOfNodeOracle(t *testing.T) {
	source := `class Probe {
 static calls=0;
 static append(a:string[]):string {Probe.calls++;a.push("x");return "x";}
 static run():string {
 const words=["","x","","x"];const nums=[0,2,0];const holes=["x"];holes[2]="x";const empty:string[]=[];
 const duplicate=words.find(x=>words.indexOf(x)!==words.lastIndexOf(x));
 const mutable=["x"];
 return words.lastIndexOf("")+":"+nums.lastIndexOf(0)+":"+holes.lastIndexOf("x")+":"+words.lastIndexOf("missing")+":"+empty.lastIndexOf("x")+":"+mutable.lastIndexOf(Probe.append(mutable))+":"+Probe.calls+":"+duplicate+":"+(duplicate===undefined);
 }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	// Do not silently ignore the optional fromIndex argument.
	negative := `class Probe {static run():number {return ["x","x"].lastIndexOf("x",0);}}`
	if got := conversionOracle(t, negative); got != "0\n" {
		t.Fatal(got)
	}
	dir := sourceProbe(t, negative)
	loaded, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, ds, err := loaded.Lower([]string{"input.ts"})
	if err != nil {
		t.Fatal(err)
	}
	refused := false
	for _, d := range ds {
		refused = refused || strings.Contains(d.Message, "fromIndex and reference elements are not lowered")
	}
	if !refused {
		t.Fatal("fromIndex was ignored", ds)
	}
}
