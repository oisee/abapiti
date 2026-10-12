package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These reads use existing nominal descriptors, not a copied object/prototype
// view. In particular, erased field-free instances need no dynamic field table.
func TestNativeConstructorNameNodeOracle(t *testing.T) {
	source := `class Base {}
class Derived extends Base {}
class Fields { value: string = "field"; }
interface Marker {}
export class Probe {
 static count: number = 0;
 static recv(): any {Probe.count++; return new Derived();}
 static name(x: any): string {return x.constructor.name;}
 static ctor(x: any): string {return x.constructor;}
 static bad(x: any): string {return x.constructor.prototype;}
 static structural(): string {return Probe.name(JSON.parse('{"value":"row"}'));}
 static live(): string {const row: any=JSON.parse('{"value":"row"}');const fields: Fields=row;return Probe.name(fields);}
 static run(): string {
  const base: Marker = new Base();
  const a: any = new Derived(); const b: any = new Derived();
  const same = a.constructor === b.constructor;
  const distinct = a.constructor !== (base as any).constructor;
  return Probe.name(base)+"/"+Probe.name(a)+"/"+Probe.name(new Fields())+"/"+Probe.ctor(JSON.parse('{"constructor":"own"}'))+"/"+same+"/"+distinct+"/"+Probe.recv().constructor.name+"/"+Probe.count;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
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
	for _, method := range []string{"structural", "live"} {
		if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe."+method+"());"), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("node", path).CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != "Object" {
			t.Fatal(method, err, string(out))
		}
		got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", method)+`())`)
		if !strings.Contains(got, "conversion domain") {
			t.Fatal(method, got)
		}
	}
	// Existing nominal metadata does not grant arbitrary function/prototype
	// properties, function calls, name writes or synthetic structural constructors.
	for _, body := range []string{
		`box(&classDescriptor{Name:str("C")}).get(str("prototype"))`,
		`(&dynamic{Tag:10,Value:&struct{}{}}).get(str("name"))`,
		`box(&classDescriptor{Name:str("C")}).put(str("name"),box(str("changed")))`,
	} {
		got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();`+body)
		if !strings.Contains(got, "unsupported") && !strings.Contains(got, "outside certified") && !strings.Contains(got, "conversion domain") {
			t.Fatal(body, got)
		}
	}
}
