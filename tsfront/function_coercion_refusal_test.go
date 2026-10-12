package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugDescriptionBindingRefusalNodeOracle(t *testing.T) {
	source := `class Symbol {static for(key:string):string{return "toString";}}
class Plain {[Symbol.for("debug.description")]():string{return "custom";}}
export class Probe {static run():string{return String(new Plain());}}`
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "custom" {
		t.Fatal(err, string(out))
	}
	prog, diags := lowerProbe(t, sourceProbe(t, source))
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") && d.Category != "skipped-computed-name" {
			t.Fatal(d)
		}
	}
	got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil{fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", "run")+`().String())`)
	if !strings.Contains(got, "outside certified") {
		t.Fatal("shadow Symbol exemption", got)
	}
}

func TestFunctionStringCoercionRefusalNodeOracle(t *testing.T) {
	source := `export class Probe {static text(x:any):string{return String(x);} static run():string{return Probe.text(()=>"ok");}}`
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != `()=>"ok"` {
		t.Fatal(err, string(out))
	}
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, `defer func(){if x:=recover();x!=nil{fmt.Print(x)}}();fmt.Print(`+goEntry("input.ts.Probe", "run")+`().String())`)
	if !strings.Contains(got, "outside certified") {
		t.Fatalf("Node function source, Go %q; callable storage escaped string coercion", got)
	}
}
