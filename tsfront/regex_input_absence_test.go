package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestRegexInputAbsentArrayNodeOracle(t *testing.T) {
	const source = `class Probe {
 static calls=0;
 static words():string[] {Probe.calls++;return ["", " "];}
 static index():number {Probe.calls++;return 2;}
 static blank(line:string):boolean {return /^\s*$/.test(line);}
 static missing(line:string):boolean {return /^undefined$/.test(line);}
 static run():string {
 const rows=["METHOD", ""];
 let next=1,counter=0;
 while(Probe.blank(rows[next]) && counter<4){counter++;next++;}
 return counter+":"+Probe.missing(rows[next])+":"+Probe.blank(rows[-1])+":"+Probe.missing(Probe.words()[Probe.index()])+":"+Probe.calls+":"+/^undefined$/.test(rows[99]);
 }
 }`
	want := conversionOracle(t, source)
	if want != "1:true:false:true:2:true\n" {
		t.Fatalf("Node: %q", want)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}
