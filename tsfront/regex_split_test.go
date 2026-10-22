package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func regexNodeOracle(t *testing.T, source string) {
	t.Helper()
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

func TestGoRegExpSplitNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static run():string {
 const patterns:RegExp[]=[/\s+/,/[\s.,:(]+/,/,/,/(,)(x)/,/a*/,/(a*)/,/(a)/,/^a/,/a$/,/\b/,/\B/,/./,/a/gi,/a/y,/$^/i];
 const inputs:string[]=["",",","a",",a,","a,,b","aaab"," a\t b ","x\u00a0y\ufeffz","😀a","a\n","éaK",",x,", "\r\n\u2028\u2029"];
 const limits:number[]=[0,1,2,3,20,-1,4294967296,4294967297,2.9,-2.9];
 let out="";
 for(const p of patterns){p.lastIndex=2;for(const s of inputs){
  const all=s.split(p);out+=all.length+":";for(let i=0;i<all.length;i++){out+=all[i]===undefined?"U;":"["+all[i]+"]";}out+="/";
  for(const lim of limits){const a=s.split(p,lim);out+=a.length+":";for(let i=0;i<a.length;i++){out+=a[i]===undefined?"U;":"["+a[i]+"]";}out+="/";}
 }out+=p.lastIndex+"!";}
 return out;
 }}`)
}

// Exercise F's bounded grammar through G's split/capture/limit implementation.
func TestGoRegExpBoundedSplitNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static run():string {
 const patterns:RegExp[]=[/a{2}/,/a{0,2}/,/a{1,3}b{1,}/,/(a{1,2})/,/(a){1}/];
 const inputs:string[]=['','a','aa','aaa','aaaa','xaaay','aaabbb','😀aa😀'];
 let out='';for(const re of patterns){re.lastIndex=3;for(const input of inputs){
 for(const limit of [0,1,2,3,20]){const parts=input.split(re,limit);out+=parts.length+':'+parts.join('|')+';';}
 out+=re.test(input)+':'+re.lastIndex+';';}}
 return out;
 }}`)
}

func TestGoDynamicRegExpCheckedSubsetNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static run():string {
 const patterns:string[]=['<abapGit\\b[^>]*\\bversion="([^"]+)"','^v\\d\\.\\d\\.\\d$','\\s+','[\\s.,:(]+','\\S+','[^\\s]+','\\bword\\B','$^'];
 const inputs:string[]=['','<abapGit version="v1.2.3">','<abapGit notversion="bad" version="v1.2.3">','<abapGitx version="bad">','<abapGit\n version="v1.2.3">','v1.2.3','v1.2.3\n','v12.2.3','a\u00a0b','wordé','word_','Kwordx','wordx'];
 let out="";for(const p of patterns){const r=new RegExp(p);for(const s of inputs){const a=r.exec(s);out+=a===null?"none;":a.index+":"+a.join("|")+";";}}
 const value='RAISE   EXCEPTION.';out+=value.replace(/\.$/,"").split(/\s+/).join("|");
 const allSpaces='\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff';
 out+=('a'+allSpaces+'b').split(/\s+/).join('|');
 out+=('a'+allSpaces+'b').split(/[\s.,:(]+/).join('|');
 out+=new RegExp('\\s+').test('\u0085')+':'+new RegExp('\\S+').test('\u0085');
 return out;
 }}`)
}

func TestGoDynamicRegExpOutsideSubsetTraps(t *testing.T) {
	source := `class Probe {static run(pattern:string,flags:string):string { return new RegExp(pattern,flags).source; }}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	main := `for _,spec:=range [][2]string{{"(a)\\1",""},{"(?=a+)",""},{"a","ui"},{"a","gg"},{"[",""},{"(a)*",""}} {func(){defer func(){v:=recover();if _,ok:=v.(trap);!ok {panic(fmt.Sprintf("expected typed trap, got %T",v))}}();` + goEntry("probe.ts.Probe", "run") + `(str(spec[0]),str(spec[1]));panic("accepted") }()};fmt.Println("refused")`
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}
