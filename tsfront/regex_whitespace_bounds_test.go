package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoRegExpWhitespaceBoundsNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const patterns:RegExp[]=[/\s/g,/\S+/,/[\s.,:(]+/,/[^\s]+/,/ (abap_true|abap_false)\s*\./i,/^[a-zA-Z]{1,3}_.*$/i,/^((\/[A-Z_\d]{3,8}\/)|[a-zA-Z0-9]{3}|CI_)\w+$/,/^(a){0,1}$/, /^a{2,3}b{1,}c{0}$/];
 const inputs:string[]=[""," ","\t\n\v\f\r","\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff","\u0085\u180e\u200b", "one, two:(three"," abap_true ."," ABAP_FALSE\ufeff.","a_foo","AbC_name","abcd_long","/ABC/NAME","/AB/NAME","/ABCDEFGHI/NAME","abcName","CI_NAME","ééé_Name","a","ab","aab","aaabbb","aaaab","😀"];
 let out="";for(const p of patterns){for(const input of inputs){p.lastIndex=0;const hit=p.exec(input);out+=hit===null?"none;":hit[0]+":"+hit.index+":"+String(hit[1])+":"+String(hit[2])+";";}}
 return out;
}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestGoRegExpErasedStringsNodeOracle(t *testing.T) {
	source := `class Probe {
 static effects=0;
 static read(value:any):any {Probe.effects++;return value;}
 static run():string {
  const patterns:any[]=['^a{1,3}$','\\bword\\b','\\s+'];
  const flags:any='i';let out='';
  for(const pattern of patterns){const re=new RegExp(Probe.read(pattern),Probe.read(flags));for(const value of ['aa',' Word ','A','xx','\ufeff']){out+=re.test(value)+':';}}
  return out+Probe.effects;
 }
 static unsupported():void {const pattern:any=JSON.parse('{}');new RegExp(pattern);}
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf(`fmt.Println(%s());func(){defer func(){if _,ok:=recover().(rangeFault);!ok{panic("erased non-string pattern must refuse")}}();%s()}()`, goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "unsupported")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
