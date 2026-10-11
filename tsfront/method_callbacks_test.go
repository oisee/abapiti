package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestMethodCallbacksNodeOracle(t *testing.T) {
	source := `class Probe {
 private prefix = "bound:";
 static log = "";
 static encode(value: string, index: number, array: string[]): string {
  return value + index + array.length;
 }
 static record(value: string): void { Probe.log += value; }
 method(value: string): string { return this.prefix + value; }
 static run(): string {
  const values = ["A", "B"];
  let obj = new Probe();
  values.forEach(Probe.record);
  const mapped = values.map(Probe.encode).join("|");
  const bound = values.map(obj.method.bind(obj)).join("|");
  const source = (): string[] => { obj = new Probe(); obj.prefix = "changed:"; return values; };
  const ordered = source().map(obj.method.bind(obj)).join("|");
  const empty: string[] = [];
  return mapped + "/" + bound + "/" + ordered + "/" + Probe.log + "/" + empty.map(Probe.encode).length;
 }
}`
	dir := sourceProbe(t, source)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.LowerWithOptions([]string{"input.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if es := hir.Verify(prog); len(es) != 0 {
		t.Fatal(es)
	}
	got := strings.TrimSpace(runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())"))
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	want, err := exec.Command("node", oracle).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestMethodCallbackRefusalsNodeOracle(t *testing.T) {
	source := `class Probe {
 value = "receiver";
 method(s: string): string { return this.value + s; }
 static run(): string { const obj = new Probe(); return ["A"].map(obj.method).join(); }
}`
	dir := sourceProbe(t, source)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, ds, err := p.LowerWithOptions([]string{"input.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil {
		t.Fatal(err)
	}
	refused := false
	for _, d := range ds {
		refused = refused || strings.Contains(d.String(), "unbound instance method callback has undefined this")
	}
	if !refused {
		t.Fatal("unbound method was silently rebound", ds)
	}
	// Node must actually lose the receiver, not call obj.method with obj.
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+`try { Probe.run(); console.log("incorrectly bound"); } catch (e) { console.log(e instanceof TypeError); }`), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", oracle).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Fatal(err, string(out))
	}
	for _, callback := range []string{"obj.method.bind(obj, \"partial\")", "obj.method.bind(new Probe())", "Probe.own"} {
		negative := strings.ReplaceAll(source, "map(obj.method)", "map("+callback+")")
		negative = strings.ReplaceAll(negative, "static run():", "static own(s: string): string { return this.name; } static run():")
		probe := integerProbe(t, negative)
		_, ds, err := probe.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
		refused := false
		for _, d := range ds {
			refused = refused || strings.Contains(d.String(), "callback")
		}
		if err != nil || !refused {
			t.Fatalf("accepted %s: %v %v", callback, err, ds)
		}
	}
}
