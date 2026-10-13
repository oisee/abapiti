package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestABAPJSONStructuralOrderNodeOracle(t *testing.T) {
	source := `interface Row {a:number;b:number;c?:string;missing?:string;}
 class Probe {static log="";static tick(n:number):number{Probe.log=Probe.log+n;return n;}
 static run():string {
 const first:Row={b:Probe.tick(1),missing:undefined,a:Probe.tick(2)};
 const second:Row={a:Probe.tick(3),b:Probe.tick(4)};
 first.c="added";first.missing="present";first.a=9;
 return JSON.stringify(first)+"|"+JSON.stringify(second)+"|"+Probe.log;
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "json-structural", strings.TrimSpace(string(want)), false)
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestABAPJSONStructuralLiveNodeOracle(t *testing.T) {
	source := `class Conf{value=0;added?:string;} interface Slot{setConfig(conf:any):void;}
    class Rule implements Slot{conf=new Conf();setConfig(conf:Conf){this.conf=conf;}write(){this.conf.added="new";}}
    class Probe{static run():string{const input:{value:number}={value:1};const rule=new Rule();const slot:Slot=rule;slot.setConfig(input);const before=JSON.stringify(rule.conf);rule.write();return before+"|"+JSON.stringify(input)+"|"+JSON.stringify(rule.conf);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "json-structural-live", strings.TrimSpace(string(want)), false)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestABAPJSONStructuralConfigNodeOracle(t *testing.T) {
	source := `interface Global{files:string;skipGeneratedGatewayClasses?:boolean;applyUnspecifiedRules?:boolean;}
 interface Config{global:Global;dependencies:string[];syntax:{version:string;globalConstants:string[]};rules:any;}
 function config(rules:any):Config{return {global:{files:"*.abap",skipGeneratedGatewayClasses:true},dependencies:[],syntax:{version:"v702",globalConstants:[]},rules};}
 class Probe{static run():string{const c=config({z:true,"10":10,"2":2,disabled:{enabled:false}});c.global.applyUnspecifiedRules=false;return JSON.stringify(c,null,2);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nprocess.stdout.write(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	// Export compact transport: ABAP string literals cannot contain raw newlines.
	compactSource := strings.Replace(source, "JSON.stringify(c,null,2)", "JSON.stringify(c)", 1)
	compact, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nprocess.stdout.write(Probe.run())")`, compactSource).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(compact))
	}
	// The pretty output is checked by Go/Node here; the existing indentation
	// fixtures execute the exact ABAP scanner. Keep this exported case compact.
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Print(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	checked := strings.Replace(compactSource, "return JSON.stringify(c);", fmt.Sprintf("return JSON.stringify(c) === %q ? \"ok\" : \"bad\";", string(compact)), 1)
	p = lowerStatementsProbe(t, map[string]string{"probe.ts": checked}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "json-structural-config", "ok", false)
}

func TestABAPJSONStructuralWideNodeOracle(t *testing.T) {
	var fields, values []string
	for i := 0; i < 45; i++ {
		fields = append(fields, fmt.Sprintf("f%02d?:number", i))
		values = append(values, fmt.Sprintf("f%02d:%d", 44-i, i))
	}
	source := `interface Wide{` + strings.Join(fields, ";") + `}class Probe{static run():string{const row:Wide={` + strings.Join(values, ",") + `};return JSON.stringify(row);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nprocess.stdout.write(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}

	// The byte oracle is long: return a Boolean equality so the exported ABAP
	// expectation stays within the source line envelope.
	checked := strings.Replace(source, "return JSON.stringify(row);", fmt.Sprintf("return JSON.stringify(row) === %q ? \"ok\" : \"bad\";", string(want)), 1)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": checked}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "json-structural-wide", "ok", false)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Print(%s())", goEntry("probe.ts.Probe", "run"))); got != "ok" {
		t.Fatal(got)
	}
}
