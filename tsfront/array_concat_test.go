package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestArrayConcatArgumentsNodeOracle(t *testing.T) {
	source := `type FixtureRow = {filename:string, contents:string};
 function runFiles(files:FixtureRow[]):string {return files.map(f=>f.filename+':'+f.contents).join('|');}
 class Probe {
 static source=[1];static first=[2];static effects='';
 static receiver():number[] {Probe.effects+='r';return Probe.source;}
 static left():number[] {Probe.effects+='l';Probe.source.push(7);return Probe.first;}
 static right():number[] {Probe.effects+='q';Probe.source.push(8);Probe.first[0]=20;return [3];}
 static fixtures(extra?:FixtureRow[]):string {
  const fixtures=[{filename:'base',contents:'xml'}];
  return runFiles(fixtures.concat([{filename:'program',contents:'abap'}],extra ?? []));
 }
 static run():string {
  const result=Probe.receiver().concat(Probe.left(),Probe.right());
  const before=result.join(',');result[0]=999;
  const fresh=Probe.source.concat();fresh.push(9);
  const a=[1,2,3];delete a[1];const copy=a.concat();
  return before+':'+Probe.source.join(',')+':'+Probe.effects+':'+fresh.join(',')+':'+(fresh===Probe.source)+':'+JSON.stringify(copy)+':'+copy.length+':'+Probe.fixtures()+':'+Probe.fixtures([{filename:'extra',contents:'more'}]);
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	// Sparse presence remains the existing explicit ABAP refusal; concat
	// itself uses only the already-supported array concat/slice operations.
	if _, err = emitABAP(t, p); err == nil || (!strings.Contains(err.Error(), "array.delete") && !strings.Contains(err.Error(), "array.has")) {
		t.Fatalf("missing sparse ABAP refusal: %v", err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
