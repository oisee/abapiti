package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// Exercise builtin dispatch and symbol-bound locals together: each switch
// label lowers a copy of the same block, including its computed initializer.
func TestDDICComputedPackedLengthBindings(t *testing.T) {
	source := `class Probe {
 static packed(kind:string,length:string):number {
  switch(kind) {
   case "DEC": case "CURR": case "QUAN": {
    const packedLength=Math.ceil((parseInt(length,10)+1)/2);
    {const packedLength=17; if(length==="shadow") return packedLength;}
    return packedLength;
   }
   default: return -1;
  }
 }
 static run():string {let result="";for(const kind of ["DEC","CURR","QUAN"]){for(const length of ["0","1","2","3","31","-2","2.5"]){result+=Probe.packed(kind,length)+";";}}return result;}
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := abap.Emit(p); err == nil || !strings.Contains(err.Error(), "number.ceil") {
		t.Fatalf("ABAP must refuse unsupported binary64 ceil: %v", err)
	}
	_, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range ds {
		if d.Category == "assume-integer" && strings.Contains(d.Message, "Math.ceil") {
			found = true
		}
	}
	if !found {
		t.Fatal("computed ceil must not become truncated integer division", ds)
	}
}

func TestBuiltinCallRejectsMergedUserGlobal(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"custom.d.ts":   `interface Math { ceil(x:number): string; }`,
		"probe.ts":      `export class Probe {static run():string {return Math.ceil(1.5);}}`,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"files":["custom.d.ts","probe.ts"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlocking(ds) {
		t.Fatal("user global overload was treated as the builtin", hir.Dump(prog))
	}
	if strings.Contains(hir.Dump(prog), "number.ceil") {
		t.Fatal("unproven global declaration reached builtin runtime", ds)
	}
}
