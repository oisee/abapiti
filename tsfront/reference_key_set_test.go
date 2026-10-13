package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestReferenceKeySetNodeOracle(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	const source = `
 interface Token { name(): string; }
 class A implements Token { name(): string { return "a"; } }
 class B implements Token { name(): string { return "b"; } }
 class Probe { static run(): string {
 const s = new Set<unknown>(); const a = new A(); const b = new B(); const token: Token = a;
 s.add(a); s.add(b); s.add(token);
 return (s.has(a) && s.has(b) && s.has(token) && !s.has(new A()) ? "identity:" : "bad:") + s.size;
 } }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if !strings.Contains(hir.Dump(p), "set<classref<builtin.object>>") {
		t.Fatal("missing reference-key proof", hir.Dump(p))
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]) + "\nconsole.log(Probe.run());")`, source)
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if string(want) != "identity:2\n" {
		t.Fatal(string(want))
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestReferenceKeySetRejectsUnprovenUses(t *testing.T) {
	for name, body := range map[string]string{
		"primitive":   `s.add(1); return s.has(new A());`,
		"alias":       `const alias = s; alias.add(1); return s.has(new A());`,
		"return":      `s.add(new A()); return s;`,
		"fluent":      `return s.add(new A());`,
		"unknown":     `const value: unknown = new A(); s.add(value); return s.has(new A());`,
		"optional":    `const value: A | undefined = undefined; s.add(value); return s.has(new A());`,
		"later-scope": `{ s.add(new A()); } s.add("x"); return s.size;`,
	} {
		t.Run(name, func(t *testing.T) {
			source := `class A {} class Probe { static run() { const s = new Set<unknown>(); ` + body + ` } }`
			p := integerProbe(t, source)
			prog, ds, err := p.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			if !hasBlocking(ds) && len(hir.Verify(prog)) == 0 {
				t.Fatal("unproven dynamic set accepted")
			}
			if strings.Contains(hir.Dump(prog), "set<classref<builtin.object>>") {
				t.Fatal("unproven set specialized")
			}
		})
	}
}
