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

// parseObserved(reg) supplies no extra arguments to a JSDoc rest parameter.
// {...unknown} resolves to unknown[], including an empty argument list.
func TestGogenA34UnknownRestNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @param {string} tag @param {...unknown} args @returns {string} */
function take(tag, ...args) { return tag + ":" + args.length; }
export class Probe {
  /** @returns {string} */ static run() {
    take("ignored");
    const a = take("empty");
    return a + "|" + take("mixed", 1, undefined, "x");
  }
}
`, true)
}

// Registry extends IRegistry with literal getObject overloads and retains
// its general overload. Upcasting keeps that general ABI; literal calls
// still get their checked result views; the upcast preserves identity.
func TestGogenA34InterfaceOverloadUpcastNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Item { name(): string { return "base"; } }
class A extends Item { onlyA(): string { return "A"; } }
class B extends Item { onlyB(): string { return "B"; } }
interface Base { get(kind: string): Item | undefined; }
interface Refined extends Base {
  get(kind: "a"): A | undefined;
  get(kind: "b"): B | undefined;
  get(kind: string): Item | undefined;
}
class Store implements Refined {
  get(kind: "a"): A | undefined;
  get(kind: "b"): B | undefined;
  get(kind: string): Item | undefined;
  get(kind: string): Item | undefined {
    if (kind === "a") return new A();
    if (kind === "b") return new B();
    return undefined;
  }
}
class Probe {
  static fromBase(r: Base): string { return r.get("b")!.name(); }
  static upcast(r: Base): Base { return r; }
  static run(): string {
    const store = new Store();
    const r: Refined = store;
    const a = r.get("a"), b = r.get("b");
    return a!.onlyA() + "|" + b!.onlyB() + "|" + Probe.fromBase(r) + "|" + (r.get("none") === undefined) + "|" + (Probe.upcast(r) === store);
  }
}`, "")
}

// Captured optional records and tagged values start as JavaScript undefined,
// and each later write remains visible through the shared binding.
func TestGogenA34UninitializedCapturedBindingNodeOracle(t *testing.T) {
	t.Run("record", func(t *testing.T) {
		gogenTSOracle(t, `type Hook = {run: () => string};
class Probe { static run(): string {
  let hook: Hook | undefined;
  const read = (): string => hook ? hook.run() : "absent";
  const first = read();
  hook = {run: () => "live"};
  const second = read();
  hook = undefined;
  return first + "|" + second + "|" + read();
} }`, "")
	})
	t.Run("tagged", func(t *testing.T) {
		gogenTSOracle(t, `class Probe { static run(): string {
  let value: unknown;
  const read = (): string => typeof value + ":" + String(value);
  const first = read();
  value = "live";
  const second = read();
  value = undefined;
  return first + "|" + second + "|" + read();
} }`, "")
	})
}

func TestGogenA34ReadonlyScalarArrayNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Scalar = string | number | boolean | null | undefined;
`, `/** @param {readonly import("./types.ts").Scalar[]} values @returns {readonly import("./types.ts").Scalar[]} */
function view(values = []) { return values; }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {string[]} */ const xs = ["a"];
    const alias = view(xs);
    const before = alias.map((value) => String(value)).join(",");
    xs[0] = "b";
    xs.push("c");
    return (alias === xs) + "|" + before + "|" + alias.map((value) => String(value)).join(",") + "|" + view().length;
  }
}
`, true)
}

func TestGogenA34ImportedReadonlyArrayNodeOracle(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"types.ts": `export type Scalar = string | number | boolean | null | undefined;`,
		"compat.d.ts": `interface RelativeIndexable<T> { at(index: number): T | undefined; }
interface ReadonlyArray<T> extends RelativeIndexable<T> {}`,
		"view.mjs": `/** @param {readonly import("./types.ts").Scalar[]} values @returns {readonly import("./types.ts").Scalar[]} */
export function view(values = []) { return values; }`,
		"probe.mjs": `import {view} from "./view.mjs";
export class Probe {
 /** @returns {string} */ static run() {
  /** @type {string[]} */ const xs = ["a"];
  const alias = view(xs);
  xs.push("b");
  return (alias === xs) + "|" + alias.map((x) => String(x)).join(",") + "|" + view().length;
 }
}`,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`,
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := exec.Command("node", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.mjs"))+`;console.log(Probe.run())`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithOptions([]string{"probe.mjs", "view.mjs"}, LowerOptions{TrapOnFailure: true})
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
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, prog); err != nil {
		t.Fatal(err)
	}
}

func TestGogenA34OptionalDiskMethodNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Disk { begin(): void; finish(n: number): void; shadow?: {probe(): void}; }
interface Session { [key: string]: unknown; disk?: Disk; }
let calls = 0, effects = 0, reads = 0;
function mark(): number { effects++; return 3; }
function getSession(session: Partial<Session> | null | undefined): Partial<Session> | null | undefined { reads++; return session; }
const disk: Disk = {begin() {}, finish(n: number) { calls += n; }};
function finish(session: Partial<Session> | null | undefined): void { getSession(session)?.disk?.finish(mark()); }
class Probe { static run(): string {
 finish(undefined); finish(null); finish({}); finish({disk});
 return calls + ":" + effects + ":" + reads;
} }
`, "")
}
