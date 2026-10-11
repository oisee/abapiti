package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestGoFreshSHA1PipelineNodeOracle(t *testing.T) {
	source := `import * as hashes from "node:crypto";
 class Probe {
 static calls=0;
 static once(s:string):string {Probe.calls++;return s;}
 static hash(s:string):string {return hashes.createHash("sha1").update(Probe.once(s)).digest("hex");}
 static run():string {return Probe.hash("")+"|"+Probe.hash("abc")+"|"+Probe.hash("æ中😀")+"|"+Probe.hash("a\0b")+"|"+Probe.calls;}
 }`
	js := strings.Replace(source, `import * as hashes from "node:crypto";`, `const hashes=require("node:crypto");`, 1)
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, js).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := abap.Emit(p); err == nil || !strings.Contains(err.Error(), "string.sha1Hex") {
		t.Fatalf("ABAP must refuse SHA-1: %v", err)
	}
}

func TestGoSHA1LocalBindingIsNotCrypto(t *testing.T) {
	source := `class Hash {update(s:string):Hash {return this;} digest(s:string):string {return "local";}}
 class Fake {createHash(s:string):Hash {return new Hash();}}
 class Probe {static run():string {const crypto=new Fake();return crypto.createHash("sha1").update("abc").digest("hex");}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "local\n" {
		t.Fatal(got)
	}
}
