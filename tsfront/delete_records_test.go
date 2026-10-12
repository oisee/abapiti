package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestDeleteRecordsNodeOracle(t *testing.T) {
	source := `class Fields { first: string = "first"; value?: string = "value"; last: string = "last"; }
class Derived extends Fields { extra: string = "extra"; }
export class Probe {
 static own(x: Fields): boolean {return true;}
 static count: number = 0;
 static recv(x: any): any {Probe.count++;return x;}
 static required(): boolean {const x: any = new Fields();return delete x.first;}
 static run(): string {
  const row: any = JSON.parse('{"first":"a","value":"v","last":"z"}');
  const view: Fields = row;
  const alias: any = row;
  const a = delete view.value;
  const b = delete alias.value;
  const native = new Fields();
  const c = delete native.value;
  const d = delete Probe.recv(native).value;
  const missing: Fields | undefined = undefined;
  const e = delete missing?.value;
  const shape = {first:"a",value: "v" as string | undefined,last:"z"};
  const f = delete shape.value;
  const own = Probe.own(native)+"/"+Probe.own(view);
  const derived: Fields = new Derived();
  delete derived.value;
  derived.value="again";
  const out = Object.keys(derived).join(",")+"/"+own+"/"+a+"/"+b+"/"+c+"/"+d+"/"+e+"/"+f+"/"+Probe.count+"/"+Object.keys(alias).join(",")+"/"+Object.keys(native).join(",")+"/"+Object.keys(shape).join(",")+"/"+(native.value === undefined)+"/"+(alias.value === undefined);
  native.value="again";
  shape.value="again";
  return out+"/"+Object.keys(native).join(",")+"/"+Object.keys(shape).join(",");
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	// Exercise the existing own-property ABI without widening TS builtin lowering.
	for _, c := range prog.Classes {
		if c.Name == "input.ts.Probe" {
			for _, m := range c.Methods {
				if m.Name == "own" {
					m.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "object.hasOwn", Type: hir.T(hir.Bool), X: hir.V("x", hir.Ref("input.ts.Fields")), Args: []*hir.Expr{hir.L(hir.T(hir.String), "value")}}})
				}
			}
		}
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(strings.Replace(source, "static own(x: Fields): boolean {return true;}", "static own(x: Fields): boolean {return x.hasOwnProperty(\"value\");}", 1)+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSpace(string(out)) {
		t.Fatalf("Go %q Node %q", got, out)
	}
	got = runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", "required")+`())`)
	if !strings.Contains(got, "delete cannot represent absence") {
		t.Fatal(got)
	}
}
