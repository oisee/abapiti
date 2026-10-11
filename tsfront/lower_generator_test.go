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
	"github.com/oisee/abapiti/tsfront/overrides"
)

func lowerGeneratorFixture(t *testing.T, source string) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"es2020","lib":["es2020"]},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, _ := overrides.New()
	prog, diags, err := p.LowerWithOverrides([]string{"probe.ts"}, r)
	if err != nil {
		t.Fatal(err)
	}
	return prog, diags
}

func TestGeneratorSuspensionAndConsumers(t *testing.T) {
	source := `export function* sequence(): Generator<number, void, undefined> { yield 2; yield* [3,4]; return; }
 export class Probe {
  public value: number = 0;
  public* values(): Generator<number, void, undefined> {
   this.value += 1; yield this.value;
   this.value += 10; yield this.value;
   this.value += 100;
  }
  public static run(): string {
   const p = new Probe(); const g = p.values();
   let out = "" + p.value;
   out += ":" + g.next().value; out += ":" + p.value;
   p.value = 20; out += ":" + g.next(undefined).value;
   const completed = g.next(); out += ":" + completed.done; out += ":" + p.value;
   out += ":" + g.next().done;
   const q = new Probe(); const early = q.values(); for (const v of early) { out += ":" + v; break; }
   out += ":" + q.value; out += ":" + early.next().done;
   const r = new Probe(); for (const v of r.values()) { if (v === 1) { continue; } out += ":" + v; }
   out += ":" + r.value;
   out += ":" + [...sequence()].join(",");
   out += ":" + Array.from(sequence()).join(",");
   return out;
  }
 }`
	prog, diags := lowerGeneratorFixture(t, source)
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs, hir.Dump(prog))
	}
	if _, _, err := abap.EmitNamed(prog); err != nil {
		t.Fatal(err)
	}
	actual := runGoHIR(t, prog, fmt.Sprintf("fmt.Print(%s())", goEntry("probe.ts.Probe", "run")))
	want := "0:1:1:30:true:130:true:1:1:true:11:111:2,3,4:2,3,4"
	oracleDir := t.TempDir()
	oracleFile := filepath.Join(oracleDir, "oracle.ts")
	if err := os.WriteFile(oracleFile, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oracle, err := exec.Command("node", oracleFile).CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v: %s", err, oracle)
	}
	if strings.TrimSpace(string(oracle)) != want {
		t.Fatalf("Node oracle: %s want %s", oracle, want)
	}
	if actual != want {
		t.Fatalf("got %s want %s", actual, want)
	}
}

func TestGeneratorRefusals(t *testing.T) {
	for _, test := range []struct{ source, message string }{
		{`export function* g(): Generator<number, number, undefined> { yield 1; return 2; }`, "only Generator<T, void, undefined>"},
		{`export function* g(): Generator<number, void, undefined> { try { yield 1; } finally {} }`, "exception handlers"},
		{`export function* g(): Generator<number, void, undefined> { const sent = yield 1; }`, "standalone statement"},
		{`export function* g(): Generator<number, void, undefined> { yield 1; } export function use(): void { g().return(); }`, "return/throw"},
	} {
		t.Run(test.message, func(t *testing.T) {
			_, ds := lowerGeneratorFixture(t, test.source)
			found := false
			for _, d := range ds {
				if d.Category == "unsupported-generator" && strings.Contains(d.Message, test.message) {
					found = true
				}
			}
			if !found {
				t.Fatal(ds)
			}
		})
	}
}

func TestGeneratorRegistrySource(t *testing.T) {
	raw, err := os.ReadFile("testdata/registryfeatures/iterators.ts")
	if err != nil {
		t.Fatal(err)
	}
	prog, diags := lowerGeneratorFixture(t, string(raw))
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs, hir.Dump(prog))
	}
	if _, _, err := abap.EmitNamed(prog); err != nil {
		t.Fatal(err)
	}
	actual := runGoHIR(t, prog, fmt.Sprintf("fmt.Print(%s())", goEntry("probe.ts.IteratorProbe", "run")))
	want := "ACB/B/0/ACC/CB/true"
	if actual != want {
		t.Fatalf("got %s want %s", actual, want)
	}
}

func TestGeneratorExceptionsDelegationAndOptionalConsumers(t *testing.T) {
	source := `export function* base(seed: number = 2): Generator<number, void, undefined> { yield seed; yield 3; }
 export function* delegate(): Generator<number, void, undefined> { yield* base(); yield 4; }
 export function* failure(): Generator<number, void, undefined> { yield 1; throw new Error("failure"); }
 export class Probe {
  public static optional(present: boolean): Generator<number, void, undefined> | undefined { if (present) { return base(); } return undefined; }
  public static twice(g: Iterable<number>): string { let out = ""; for(const value of g) { out += "" + value; } out += "/"; for(const value of g) { out += "" + value; } return out; }
  public static first(g: Generator<number, void, undefined>): number { for (const value of g) { return value; } return 0; }
  public static run(): string {
   let out = [...delegate()].join(",");
   out += ":" + Probe.twice([2,3]); out += ":" + Probe.twice(base());
   const d = delegate(); for (const value of d) { break; } out += ":" + d.next().done;
   const f = failure(); out += ":" + f.next().value;
   try { f.next(); } catch (e) { out += ":caught"; }
   out += ":" + f.next().done;
   const g = base(); try { for (const value of g) { throw new Error("consumer"); } } catch(e) { out += ":consumer"; }
   out += ":" + g.next().done;
   const handled = base(); for (const value of handled) { try { throw new Error("handled"); } catch(e) { out += ":handled"; } out += ":" + value; }
   const h = base(); out += ":" + Probe.first(h); out += ":" + h.next().done;
   for (const value of Probe.optional(false) || []) { out += ":unexpected"; }
   for (const value of Probe.optional(true) || []) { out += ":" + value; }
   return out;
  }
 }`
	prog, diags := lowerGeneratorFixture(t, source)
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs, hir.Dump(prog))
	}
	if _, _, err := abap.EmitNamed(prog); err != nil {
		t.Fatal(err)
	}
	actual := runGoHIR(t, prog, fmt.Sprintf("fmt.Print(%s())", goEntry("probe.ts.Probe", "run")))
	dir := t.TempDir()
	file := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(file, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oracle, err := exec.Command("node", file).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, oracle)
	}
	if actual != strings.TrimSpace(string(oracle)) {
		t.Fatalf("Go %s Node %s", actual, oracle)
	}
}
