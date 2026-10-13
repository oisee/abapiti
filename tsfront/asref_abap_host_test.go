package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// Reproduce the parsed nested-object boundary without Chai or JSON serialization.
// Exercise the actual TS lowering and emitted ABAP, not a hand-built adapter.
func TestParsedNestedObjectABAPHost(t *testing.T) {
	const source = `interface Child {flag:boolean; optional?:string;}
 interface Config {child:Child;}
 class ParsedConfig {private readonly config:Config; constructor(text:string) {this.config=JSON.parse(text);} read():boolean {return this.config.child.flag;}}
 class Probe {
 static view(input:any):Config {return input as Config;}
 static has(input:object,key:string):boolean {return key in input;}
 static run():string {
 const input=JSON.parse('{"child":{"flag":true,"extra":"kept"}}');
 const conf:Config=input;
 const first=conf.child;const again=Probe.view(input).child;
 const minimal=new ParsedConfig('{"child":{"flag":true}}').read();
 first.flag=false;const seen=input.child.flag;
 input.child.flag=true;
 const raw:any=first;
 const absentBefore=!Probe.has(first,"optional");
 first.optional=undefined;
 const present=Probe.has(first,"optional");
 const absent=first.optional===undefined;
 input.child=JSON.parse('{"flag":false,"extra":"replacement"}');
 const fresh=conf.child;fresh.flag=true;
 const rawFresh:any=fresh;
 return minimal+":"+(first===again)+":"+seen+":"+first.flag+":"+absentBefore+":"+present+":"+absent+":"+(first!==fresh)+":"+input.child.flag+":"+raw.extra+":"+rawFresh.extra;
 }
 static invalid():void {const input=JSON.parse('{"child":42}');const conf:Config=input;const flag=conf.child.flag;}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("invalid child escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "invalid")
}
