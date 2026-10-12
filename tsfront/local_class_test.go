package tsfront

import (
	"github.com/oisee/abapiti/hir/abap"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalClassIdentityNodeOracle(t *testing.T) {
	source := `interface Marker {get(): string;}
export class Probe {
 static make(): any {
  class Local implements Marker {get(): string {return "ok";}}
  return Local;
 }
 static direct(): string {
  class Local {get(): string {return "direct";}}
  const x=new Local();
  return x.get()+"/"+(x instanceof Local)+"/"+(x as any).constructor.name;
 }
 static run(): string {
  const a=Probe.make();const b=Probe.make();
  const x=new a();const y=new b();
  return x.get()+"/"+(a!==b)+"/"+(x.constructor===a)+"/"+(y.constructor===b)+"/"+(x instanceof a)+"/"+(x instanceof b)+"/"+x.constructor.name;
 }
}`
	// Calls on arbitrary any are still refused; invoke the declared Marker slot.
	source = strings.Replace(source, `const x=new a();const y=new b();`, `const x: Marker=new a();const y: Marker=new b();`, 1)
	source = strings.ReplaceAll(source, `x.constructor`, `(x as any).constructor`)
	source = strings.ReplaceAll(source, `y.constructor`, `(y as any).constructor`)
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	if _, err := abap.Emit(prog); err == nil {
		t.Fatal("ABAP accepted lexical descriptors")
	}
	path := filepath.Join(t.TempDir(), "oracle.ts")
	for _, method := range []string{"run", "direct"} {
		got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", method)+"().String())")
		if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe."+method+"());"), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("node", path).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		if got != strings.TrimSpace(string(out)) {
			t.Fatalf("Go %q Node %q", got, out)
		}
	}
}

func TestLocalClassCaptureRefusals(t *testing.T) {
	for _, body := range []string{
		`const captured="value";class Local {get(): string {return captured;}}`,
		`class Local {field:string="value";}`,
		`class Local {constructor(){}}`,
		`class Local {static get():string{return "value";}}`,
		`class Base{};class Local extends Base {}`,
		`class Local {get():any{return Local;}}`,
	} {
		requireDiagnostic(t, sourceProbe(t, `export class Probe {run():void {`+body+`}}`), "unsupported-local-class")
	}
}
