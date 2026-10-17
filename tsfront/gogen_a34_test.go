package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A mutable destructured option is one shared binding across the callbacks
// before and after reassignment, including when the option is absent.
func TestGogenA34CapturedDestructuredSessionNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Session { [key: string]: unknown; name: string }
function audit(run: () => void): void { run(); }
function finish(session: Partial<Session> | null | undefined, log: string[]): void {
  log.push(session ? String(session.name) : "absent");
}
function compile({session}: {session?: Partial<Session> | null}): string {
  const log: string[] = [];
  const first = () => finish(session, log);
  audit(first);
  session = {name: "new"};
  audit(() => finish(session, log));
  audit(first);
  return log.join(",");
}
class Probe { static run(): string {
  return compile({session: {name: "old"}}) + "|" + compile({}) + "|" + compile({session: null});
} }
`, "RequireObjectCoercible")
}

// An unreachable failed synthetic signature must not remove an unrelated
// enum's ID-zero namespace slot. Compare that unaffected program with Node.
func TestGogenA34SignatureRepairEnumNodeOracle(t *testing.T) {
	source := `enum Severity { Error = "Error", Warning = "Warning", Info = "Info" }
class Probe { static run(): string { return Object.keys(Severity).join(",") + ":" + Object.values(Severity).join(","); } }`
	wantBytes, err := exec.Command("node", "-e", `const ts=require(process.argv[2]);eval(ts.transpileModule(process.argv[1],{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText+"\nconsole.log(Probe.run())")`, source, typescriptModule(t)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(wantBytes))
	}
	want := string(wantBytes)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	p.Interfaces = append(p.Interfaces, &hir.Interface{Name: "unused.synthetic", Methods: []*hir.Method{{Name: "bad", Result: hir.Ref("missing")}}})
	(&lowerer{out: p}).repairFailedBodies()
	if errors := hir.Verify(p); len(errors) != 0 {
		t.Fatal(errors)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
