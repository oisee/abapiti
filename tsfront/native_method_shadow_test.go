package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeMethodGetterShadowRefusalNodeOracle(t *testing.T) {
	source := `class Base { read(): string {return "base";} }
class Child extends Base {get read(): string {return "not callable";}}
class OnlyUnsafe {get attr():string{return "attr";} unique():string{return "only";}}
export class Probe {
 static call(x:any): string {return x.read();}
 static run():string{return Probe.call(new Child());}
 static callUnique(x:any):string{return x.unique();}
 static emptyCatalogue():string{return Probe.callUnique(new OnlyUnsafe());}
}`
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\ntry {Probe.run();}catch(e){console.log(e.name);}"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "TypeError" {
		t.Fatal(err, string(out))
	}
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", "run")+`().String())`)
	if !strings.Contains(got, "outside known concrete classes") {
		t.Fatalf("Node TypeError, Go %q; getter shadow did not refuse", got)
	}
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.emptyCatalogue());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "only" {
		t.Fatal(err, string(out))
	}
	got = runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", "emptyCatalogue")+`().String())`)
	if !strings.Contains(got, "outside known concrete classes") {
		t.Fatal("uncertified empty native catalogue", got)
	}
}
