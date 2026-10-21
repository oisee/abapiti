package tsfront

import (
	"fmt"
	"testing"
)

// The sparse runtime patches Array.prototype.join by its exact body text; a
// hole must join as the empty string, not as the zero value.
func TestSparseJoinHoleNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const a=[1,2,3];delete a[1];
 const s=new Array<number>(3);s[2]=4;
 return a.join(",")+"|"+s.join("-")+"|"+a.join();}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
