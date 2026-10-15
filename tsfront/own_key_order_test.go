package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Own-key order of class instances, the guard for a lazy key-order
// representation: Object.keys, for-in and JSON.stringify must keep Node's
// order (observed through JSON.stringify and Object.keys) across constructor
// order, subclass initializers after super(), an overridden method writing a
// subclass field from the base constructor, undefined writes, re-assignment
// and delete/re-add. Computed keys, spreads and Object.assign onto nominal
// instances are refused by the lowering today, so they are not covered here.
const ownKeyOrderSource = `class Base {
 b1?: number;
 b2 = 2;
 constructor(early: boolean) { this.b1 = 1; if (early) { this.hook(); } }
 hook(): void {}
}
class Sub extends Base {
 s1?: string;
 s2 = "two";
 s3?: boolean;
 constructor(early: boolean) { super(early); this.s1 = "one"; }
 hook(): void { this.s3 = true; }
}
class Plain {
 z?: number;
 a?: number;
 constructor(swap: boolean) { if (swap) { this.a = 1; this.z = 2; } else { this.z = 2; this.a = 1; } }
}
class Probe {
 static run(): string {
  const out: string[] = [];
  out.push(JSON.stringify(new Plain(false)));
  out.push(JSON.stringify(new Plain(true)));
  out.push(JSON.stringify(new Base(false)));
  out.push(JSON.stringify(new Sub(false)));
  out.push(JSON.stringify(new Sub(true)));
  const u = new Plain(true);
  u.z = undefined;
  u.a = 5;
  out.push(JSON.stringify(u) + Object.keys(u).join(","));
  const d = new Plain(false);
  delete d.z;
  d.z = 9;
  out.push(JSON.stringify(d));
  return out.join("\n");
 }
}`

func TestOwnKeyOrderNodeOracle(t *testing.T) {
	source := ownKeyOrderSource
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go:\n%s\nNode:\n%s", got, want)
	}
}

// abaplint compiles with target es2017: declared fields are not defined up
// front; keys appear in assignment order. The oracle is TypeScript's own
// es2017 output run by Node.
func TestOwnKeyOrderES2017NodeOracle(t *testing.T) {
	ts := typescriptModule(t)
	script := `const ts=require(process.argv[2]);const js=ts.transpileModule(process.argv[1],{compilerOptions:{target:ts.ScriptTarget.ES2017,module:ts.ModuleKind.CommonJS}}).outputText;eval(js+"\nconsole.log(Probe.run())")`
	want, err := exec.Command("node", "-e", script, ownKeyOrderSource, ts).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerProbeTarget(t, "ES2017", map[string]string{"probe.ts": ownKeyOrderSource}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go:\n%s\nNode (tsc es2017):\n%s", got, want)
	}
}

func typescriptModule(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	typescript := filepath.Join(home, "dev/abaplint/node_modules/typescript")
	if _, err := os.Stat(typescript); err != nil {
		t.Skip("upstream node_modules not installed: " + typescript)
	}
	return typescript
}

// lowerProbeTarget is lowerStatementsProbe with an explicit compile target.
func lowerProbeTarget(t *testing.T, target string, sources map[string]string, files []string) *hir.Program {
	t.Helper()
	dir := t.TempDir()
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"`+target+`"},"include":["*.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Errorf("lower: %s", d)
		}
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs[0])
	}
	return prog
}
