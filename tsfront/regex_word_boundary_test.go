package tsfront

import (
 "fmt"
 "os/exec"
 "testing"
)

func TestGoRegExpWordBoundaryNodeOracle(t *testing.T) {
 source:=`class Probe {static run():string {
 const patterns:RegExp[]=[/\bword\b/,/\Bword\B/,/\bWORD\b/i,/\b/,/\B/,/(\ba)(b\B)?/];
 for(const attribute of ["version","serializer_version"]) {
  patterns.push(new RegExp('<abapGit\\b[^>]*\\b'+attribute+'="([^"]+)"'));
 }
 const inputs:string[]=["","word"," word ","swordfish","word_","_word","éwordé","Kwordſ","😀word😀","word\n","word\r","word\u2028","word\u2029","ab","abc","<abapGit version=\"v1.0.0\" serializer_version=\"v2.0.0\">","<abapGitX version=\"wrong\">","<abapGit serializer_version=\"v2.0.0\">","<abapGit xversion=\"wrong\" version=\"right\">","<abapGit version=\"first\" version=\"last\">","<abapGit\nversion=\"😀\">","<abapGit version=\"\">","<abapGit version=\"a\nb\">"];
 let out="";for(const p of patterns){for(const input of inputs){const hit=p.exec(input);out+=hit===null?"none;":hit[0]+":"+hit.index+":"+String(hit[1])+":"+String(hit[2])+";";}}
 // lastIndex must not redefine the left boundary of the input.
 const global=/\bword\b/g, sticky=/\bword\b/y;
 for(const p of [global,sticky]){for(const input of ["sword"," word"]){p.lastIndex=1;const hit=p.exec(input);out+=(hit===null?"none":hit[0]+":"+hit.index)+":"+p.lastIndex+";";}}
 return out;
}}`
 want,err:=exec.Command("node","--disable-warning=ExperimentalWarning","-e",`eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`,source).CombinedOutput()
 if err!=nil{t.Fatal(err,string(want))}
 p:=lowerStatementsProbe(t,map[string]string{"probe.ts":source},[]string{"probe.ts"})
 got:=runGoHIR(t,p,fmt.Sprintf("fmt.Println(%s())",goEntry("probe.ts.Probe","run")))
 if got!=string(want){t.Fatalf("Go %q Node %q",got,want)}
}
