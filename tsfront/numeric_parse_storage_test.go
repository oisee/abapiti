package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestNumericParseRecordStorageNodeOracle(t *testing.T) {
	const source = `interface TextEntry {entry:string;maxLength:number;}
 class Probe {
 static calls=0;
 static input():any {Probe.calls++;return undefined;}
 static run():string {
 const text:TextEntry={entry:"Hello",maxLength:parseInt(Probe.input(),10)};
 const finite:TextEntry={entry:"ok",maxLength:parseInt(" 42tail",10)};
 return Object.keys(text).join(",")+":"+JSON.stringify(text)+":"+JSON.stringify(finite)+":"+Probe.calls;
 }
 }`
	want := conversionOracle(t, source)
	if want != "entry,maxLength:{\"entry\":\"Hello\",\"maxLength\":null}:{\"entry\":\"ok\",\"maxLength\":42}:1\n" {
		t.Fatalf("Node: %q", want)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}
