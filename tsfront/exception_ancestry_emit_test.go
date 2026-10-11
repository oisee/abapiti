package tsfront

import (
	"fmt"
	"testing"
)

func TestGoCatchDerivedError(t *testing.T) {
	source := `class FailCombinatorError extends Error {}
 class Probe {
 static fail():void {throw new FailCombinatorError();}
 static run():string {
  try {Probe.fail();}catch(err){if(err instanceof FailCombinatorError){return "caught";}throw err;}
  return "missed";
 }
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "caught\n" {
		t.Fatal(got)
	}
}
