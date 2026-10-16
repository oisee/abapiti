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

func TestNumericParseAnonymousRecordCapture(t *testing.T) {
	const source = `interface TextElements {[key:string]:{entry:string;maxLength:number};}
 class Probe {
 static calls=0;
 static input():any {Probe.calls++;return undefined;}
 static run():string {
 const texts:TextElements={};
 texts["I"]={entry:"Hello",maxLength:parseInt(Probe.input(),10)};
 return texts["I"].entry+":"+Probe.calls;
 }
 }`
	want := conversionOracle(t, source)
	if want != "Hello:1\n" {
		t.Fatalf("Node: %q", want)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	text := files[names.Get("probe.ts.Probe")+".clas.abap"]
	if !strings.Contains(text, "=>int_optional(") || strings.Contains(text, "range_parse_safe_prefix") {
		t.Fatal("anonymous record capture evaluated parseInt before its tagged destination")
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}
