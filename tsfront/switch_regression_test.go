package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestSwitchGroupedLabelsMatchNode(t *testing.T) {
	source := `export class Probe {
 static trace: string = "";
 static state: string = "a";
 static subject(): string { Probe.trace += "S"; return "b"; }
 static label(x: string): string { Probe.trace += x; return x; }
 static mutate(): string { Probe.state = "b"; return "b"; }
 static lex(text: string): string {
  let build = "";
  const tokens: string[] = [];
  for (let i = 0; i < text.length; i++) {
   const next = text.charAt(i);
   switch (next) {
    case " ": case "\t": case "\n":
     if (build.length > 0) { tokens.push(build); build = ""; }
     break;
    case "{": case "}": case "=": case "<": case ">":
     if (build.length > 0) { tokens.push(build); build = ""; }
     tokens.push(next);
     break;
    default: build += next; break;
   }
  }
  if (build.length > 0) { tokens.push(build); }
  return tokens.join("|");
 }
 static middle(x: string): string {
  switch (x) {
   case "a": return "a";
   default: return "fallback";
   case "b": case "c": return "bc";
   case "b": return "duplicate";
  }
  return "outside";
 }
 static groupedDefault(x: string): string {
  switch (x) { case "a": default: case "b": return "shared"; case "c": return "c"; }
  return "outside";
 }
 static order(): string {
  let result = "";
  switch (Probe.subject()) {
   default: result = "default"; break;
   case Probe.label("a"): case Probe.label("b"): result = "matched"; break;
   case Probe.label("c"): result = "wrong"; break;
  }
  switch (Probe.state) { case Probe.mutate(): result += "wrong"; break; case "a": result += "frozen"; break; }
  switch (Probe.subject()) {}
  return result + "/" + Probe.trace;
 }
 static local(): string {
  let x = 1;
  switch (x) { case x++ + 1: return "wrong"; case 1: return "frozen"; default: return "wrong"; }
 }
 static loops(): string {
  let result = "";
  for (let i = 0; i < 3; i++) {
   switch (i) {
    case 0: case 1: continue;
    default: while (result.length < 3) { result += "x"; break; } break;
   }
   result += "y";
  }
  return result;
 }
 static run(): string {
  return Probe.lex("define view foo as select from bar { field }") + "/" +
   Probe.lex("a=b\tc<d\ne>f") + "/" + Probe.middle("a") + "/" + Probe.middle("b") + "/" +
   Probe.middle("c") + "/" + Probe.middle("z") + "/" + Probe.groupedDefault("a") + "/" +
   Probe.groupedDefault("z") + "/" + Probe.groupedDefault("c") + "/" + Probe.order() + "/" + Probe.local() + "/" + Probe.loops();
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	// Both backends consume the corrected structured HIR (no Go-only workaround).
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	oracle := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(oracle, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", oracle).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
}

func TestSwitchRefusesUnmodeledControlFlow(t *testing.T) {
	for _, body := range []string{
		`case "a": result += "a"; case "b": result += "b"; break;`,
		`case "a": if (result === "") { break; } result += "a"; break;`,
		`case "a": { result += "a"; break; }`,
		`case result: let result = "shadow"; break;`,
		`case "a": let local = "a"; break; case "b": result = local; break;`,
	} {
		source := `export class Probe { static run(x: string): string { let result = ""; while (x.length > 0) { switch (x) { ` + body + ` } return result; } return result; } }`
		requireDiagnostic(t, sourceProbe(t, source), "unsupported-statement")

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		config := filepath.Join(dir, "tsconfig.json")
		if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := Load(config)
		if err != nil {
			t.Fatal(err)
		}
		prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
		if err != nil || hasBlocking(ds) {
			t.Fatalf("partial lowering: %v %v", ds, err)
		}
		if errors := hir.Verify(prog); len(errors) > 0 {
			t.Fatal(errors)
		}
		trapped := false
		for _, c := range prog.Classes {
			for _, m := range c.Methods {
				if m.Name == "run" {
					trapped = isFailureBody(m.Body)
				}
			}
		}
		if !trapped {
			t.Fatal("unsupported switch produced a partial answer instead of a whole-body trap")
		}
		if _, err := abap.Emit(prog); err != nil {
			t.Fatal(err)
		}
		main := `func(){defer func(){if recover()==nil{panic("missing switch trap")};fmt.Print("trapped")}();` + goEntry("probe.ts.Probe", "run") + `(str("a"))}()`
		if got := runGoHIR(t, prog, main); got != "trapped" {
			t.Fatal(got)
		}
	}
}
