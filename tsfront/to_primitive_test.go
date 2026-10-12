package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestObjectStringCoercionNodeOracle(t *testing.T) {
	source := `class Plain { value: string = "field"; }
class Override { toString(): string {Probe.count++;return "custom";} }
class Child extends Override {}
interface Marker {}
class ValueHook {valueOf(): number {return 7;} toString(): string {return "string";}}
class SymbolHook {[Symbol.toPrimitive](hint: string): string {return hint;}}
export class Probe {
 static count: number = 0;
 static recv(): Marker {Probe.count++;return new Child();}
 static dynamic(x:any): string {return String(x);}
 static valueHook(): string {return ""+new ValueHook();}
 static symbolHook(): string {return String(new SymbolHook());}
 static ownHook(): string {return Probe.dynamic(JSON.parse('{"toString":"shadow"}'));}
 static run(): string {
  const plain=new Plain(); const child: Marker=new Child();
  const a:any=JSON.parse('["a",null,[1,2],{},true]');
  const b: string[]=["x","y"];
  return String(plain)+"/"+String(child)+"/"+String(Probe.recv())+"/"+String(a)+"/"+String(b)+"/"+String(JSON.parse('{}'))+"/"+String(undefined)+"/"+String(null)+"/"+Probe.count;
 }
 static template(): string {return ` + "`" + `${new Child()}/${["a","b"]}` + "`" + `;}
 static plus(): string {return "left:"+new Child();}
}`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower([]string{"input.ts"})
	if err != nil {
		t.Fatal(err)
	}
	computed := 0
	for _, d := range diags {
		if d.Category == "skipped-computed-name" {
			computed++
			continue
		}
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if computed != 1 {
		t.Fatalf("computed hook diagnostic count %d", computed)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	path := filepath.Join(t.TempDir(), "oracle.ts")
	for _, method := range []string{"run", "template", "plus"} {
		got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", method)+"().String())")
		if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe."+method+"());"), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("node", path).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		if got != strings.TrimSpace(string(out)) {
			t.Fatalf("%s Go %q Node %q", method, got, out)
		}
	}
	for method, want := range map[string]string{"valueHook": "7", "symbolHook": "string", "ownHook": "TypeError"} {
		if err := os.WriteFile(path, []byte(source+"\ntry {console.log(Probe."+method+"());}catch(e){console.log(e.name);}"), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("node", path).CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != want {
			t.Fatal(method, err, string(out))
		}
		got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", method)+`())`)
		refusal := "outside certified"
		if method == "ownHook" {
			// The merged record guard refuses own hooks before native coercion.
			refusal = "record string conversion requires the original default Object coercion hooks"
		}
		if !strings.Contains(got, refusal) {
			t.Fatal(method, got)
		}
	}
}
