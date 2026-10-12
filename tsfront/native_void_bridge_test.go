package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestErasedNativeVoidValueBridgeNodeOracle(t *testing.T) {
	source := `interface I {getConfig():void;}
class actual {tag="actual";}
abstract class Base<T extends actual> implements I {abstract getConfig():T;}
class Derived extends Base<actual> {private conf=new actual();getConfig():actual{return this.conf;}}
export class Probe {static call(x:any):string{return x.getConfig().constructor.name;}static run():string{return Probe.call(new Derived());}}`
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "actual" {
		t.Fatal(err, string(out))
	}
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	if got != "actual" {
		t.Fatal("erased void value bridge", got)
	}
}

func TestErasedNativeVoidUnknownResultRefusalNodeOracle(t *testing.T) {
	source := `const f:()=>void=()=>7;
class Unknown {read():void{return f();}}
export class Probe {static call(x:any):string{return String(x.read());}static run():string{return Probe.call(new Unknown());}}`
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "7" {
		t.Fatal(err, string(out))
	}
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil{fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", "run")+`().String())`)
	if !strings.Contains(got, "outside known concrete classes") {
		t.Fatal("uncertified void return escaped", got)
	}
}
