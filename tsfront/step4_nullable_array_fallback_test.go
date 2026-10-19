package tsfront

import (
	"fmt"
	"testing"
)

func TestStep4NullableArrayFallbackNodeOracle(t *testing.T) {
	source := `class Probe {
 static read(value:string,owner:string|undefined):string {
  const named=/^(\w+)=>(\w+)$/.exec(value) ?? (/^\w+$/.test(value) && owner ? [value,owner,value] : null);
  return named ? named[1] : "none";
 }
 static pick(flag:boolean,values:string[]):string {
  const picked=flag ? values : null;
  return picked === null ? "none" : picked === values ? "same" : "copy";
 }
 static run():string {return Probe.read("a=>b",undefined)+"|"+Probe.read("a","owner")+"|"+Probe.read("a",undefined)+"|"+Probe.read("[]","owner")+"|"+Probe.pick(true,["x"])+"|"+Probe.pick(true,[])+"|"+Probe.pick(false,[]);}
}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
