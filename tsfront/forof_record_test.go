package tsfront

import (
	"fmt"
	"testing"
)

// Index signatures describe the present value, whereas map.get also carries
// absence. for-of must consume the array view without copying its elements.
func TestForOfRecordArray(t *testing.T) {
	source := `class Item { constructor(public text: string) {} }
 class Probe {
 static calls: number = 0;
 static values(): {[key: string]: Item[]} {
   Probe.calls += 1;
   return {items: [new Item("a"), new Item("b")]};
 }
 static run(): string {
   let result = "";
   for (const item of Probe.values()["items"]) { result += item.text; }
   const numbers: {[key: string]: number[]} = {items: [2, 3], empty: []};
   let sum = 0;
   for (const value of numbers["items"]) { sum += value; }
   for (const value of numbers["empty"]) { sum += value; }
   return result + ":" + sum + ":" + Probe.calls;
 }
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "ab:5:1\n" {
		t.Fatal(got)
	}
}
