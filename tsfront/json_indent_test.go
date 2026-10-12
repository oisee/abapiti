package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONLiteralIndentNodeOracle(t *testing.T) {
	source := `export class Probe {
 static order:string="";
 static value():any {Probe.order+="V";return JSON.parse('{"z":[{},[],{"text":"\\\"{},:[]\\\\","n":1e-7}],"2":"two","1":"one","lone":"\\ud800"}');}
 static run():string {
  const a=JSON.stringify(Probe.value(),null,2);
  const value=Probe.value();
  const cases=[a,JSON.stringify(value,null,0),JSON.stringify(value,null,2.9),JSON.stringify(value,null,99),JSON.stringify(value,undefined,"\t"),JSON.stringify(value,null,"😀0123456789"),JSON.stringify([[],{},true,null,0,"{}"],null,2),JSON.stringify("scalar",null,2),JSON.stringify({a:undefined,b:1},null,2)];
  return cases.join("/\n")+"/"+Probe.order;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	dir := t.TempDir()
	path := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSuffix(string(out), "\n") {
		t.Fatalf("Go %q Node %q", got, out)
	}
}

func TestJSONIndentRefusesReplacerAndNonliteralSpace(t *testing.T) {
	for _, source := range []string{
		`export class Probe {run():string{return JSON.stringify({a:1},["a"],2);}}`,
		`export class Probe {run(space:number):string{return JSON.stringify({a:1},null,space);}}`,
		`export class Probe {run(undefined:any[]):string{return JSON.stringify({a:1},undefined,2);}}`,
	} {
		requireDiagnostic(t, sourceProbe(t, source), "unsupported-json")
	}
}
