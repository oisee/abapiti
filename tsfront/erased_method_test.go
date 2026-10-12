package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestErasedUnionMethodNodeOracle(t *testing.T) {
	source := `interface Product {a(): string;}
class ProductA implements Product { a(): string {return "a";} }
class ProductB { b(): string {return "b";} }
class A { get(): Product {Probe.trace += "A";return new ProductA();} getChildren(): (A|B)[] {return [new B()];} }
class B { get(): ProductB {Probe.trace += "B";return new ProductB();} getChildren(): (A|B)[] {return [];} }
export class Probe {
 static trace: string = "";
 static receiver(): (A | B)[] {Probe.trace += "R";return [new B(),new A()];}
 static run(): string {
  const a=Probe.receiver().filter(c => c.get().constructor.name === "ProductA");
  return a.length+"/"+Probe.trace+"/"+a[0].get().constructor.name+"/"+Probe.walk(new A());
 }
 static walk(x:any): string {
  const k=x.get?.()?.constructor?.name;
  let ret=String(k);
  for(const c of x.getChildren?.() ?? []) {ret += Probe.walk(c);}
  return ret;
 }
 static bad(): string {return Probe.walk(JSON.parse('{"get":"not callable"}'));}
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	if !strings.Contains(hir.Dump(prog), "dynamic.call0") {
		t.Fatal("missing concrete erased dispatch")
	}
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSpace(string(out)) {
		t.Fatalf("Go %q Node %q", got, out)
	}
	if err := os.WriteFile(path, []byte(source+"\n"+`try {Probe.bad();}catch(e){console.log(e.name);}`), 0600); err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "TypeError" {
		t.Fatal(err, string(out))
	}
	got = runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil{fmt.Print(x)}}();`+goEntry("input.ts.Probe", "bad")+`()`)
	if !strings.Contains(got, "native method call requires a concrete class") {
		t.Fatal(got)
	}
}
