package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGogenWeakMapNodeOracle(t *testing.T) {
	runMjsOracle(t, `class Key { constructor() { this.n = 1; } }
export class Probe {
 static run() {
  const a = new Key(), b = new Key();
  /** @type {WeakMap<Key, string>} */ const m = new WeakMap();
  m.set(a, "a").set(b, "b");
  const before = m.get(a) + ":" + m.get(b) + ":" + m.has(new Key());
  return before + ":" + m.delete(a) + ":" + m.delete(a) + ":" + (m.get(a) === undefined) + ":" + m.has(b);
 }
}`, false)
}

func TestGogenDateNowNodeOracle(t *testing.T) {
	runMjsOracle(t, `export class Probe { static run() { const n = Date.now(); return n > 1700000000000 && n < 2100000000000 && n === Math.ceil(n); } }`, false)
}

func TestGogenFunctionParameterNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @param {(x: number, y: number) => number} place */
function invoke(place) { return place(2, 3); }
export class Probe { static run() { return invoke((x, y) => x * 10 + y); } }`, true)
}

func TestGogenMapFieldNodeOracle(t *testing.T) {
	runMjsOracle(t, `class Program {
 constructor() {
 /** @type {Map<string, number>} */
 this.functions = new Map();
 }
}
/** @param {Program} p */ function store(p) { p.functions.set("x", 7); return "v=" + p.functions.get("x"); }
export class Probe { static run() { return store(new Program()); } }`, true)
}

func TestGogenDestructuredFunctionNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @typedef {{place: (x: number, y: number) => number}} Renderers */
/** @param {Renderers} h */ function invoke(h) { const {place} = h; return place(2, 3); }
export class Probe { static run() { return invoke({place: (x, y) => x * 10 + y}); } }`, true)
}

func TestGogenFunctionEvaluationNodeOracle(t *testing.T) {
	runMjsOracle(t, `export class Probe {
 static run() {
  /** @type {(x: number, y: number) => number} */ let f = (x, y) => x * 10 + y;
  let first = 2;
  function replace() { first = 9; f = (x, y) => x * 100 + y; return 3; }
  const value = f(first, replace());
  return value + ":" + f(2, 3);
 }
}`, true)
}

func TestGogenWeakPrimitiveNodeOracle(t *testing.T) {
	source := `class Key { n = 1; }
class Probe {
 static run(): string {
  const m = new WeakMap<any, string>();
  let errors = "";
  for (const key of [1, "x", true, null, undefined]) {
   try { m.set(key, "bad"); errors += "miss"; }
   catch (e) { errors += (e instanceof TypeError) + ":" + (e instanceof Error) + ";"; }
  }
  const a = new Key(); m.set(a, "ok");
  return errors + m.has(1) + ":" + m.delete("x") + ":" + (m.get(null) === undefined) + ":" + m.get(a);
 }
}`
	gogenTSOracle(t, source, "weak.")
}

func TestGogenWeakSetNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Key { n = 1; } class Probe { static run(): string { const a = new Key(), b = new Key(); const s = new WeakSet<Key>(); s.add(a).add(b); return s.has(a) + ":" + s.has(new Key()) + ":" + s.delete(a) + ":" + s.delete(a) + ":" + s.has(b); } }`, "weak.")
}

func gogenTSOracle(t *testing.T, source, refusal string) {
	t.Helper()
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	_, err := emitABAP(t, p)
	if refusal == "" {
		if err != nil {
			t.Fatal(err)
		}
	} else if err == nil || !strings.Contains(err.Error(), refusal) || !strings.Contains(err.Error(), "probe.ts:") {
		t.Fatalf("wanted located %s refusal, got %v", refusal, err)
	}
}

func TestGogenMapAliasFieldNodeOracle(t *testing.T) {
	gogenTSOracle(t, `type Functions = Map<string, {name: string}>;
class Program { functions: Functions = new Map(); }
class Probe { static store(p: Program): string { p.functions.set("x", {name: "ok"}); return p.functions.get("x")!.name; } static run(): string { return Probe.store(new Program()); } }`, "")
}

func TestGogenMapAliasIntersectionNodeOracle(t *testing.T) {
	gogenTSOracle(t, `type Row = {name: string}; type Rows = Map<string, Row & {readonly size: number}>;
class Program { functions: Rows = new Map(); }
class Probe { static run(): string { const p = new Program(); p.functions.set("x", {name: "ok", size: 2}); return p.functions.get("x")!.name; } }`, "")
}

func TestGogenInheritedRendererNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Base { readonly label: string; render(x: number): string; }
interface Renderers extends Base { place(x: number): string; }
class Probe { static invoke(h: Renderers): string { const {render, place} = h; return render(2) + place(3); } static run(): string { return Probe.invoke({label: "r", render: (x) => "r" + x, place: (x) => "p" + x}); } }`, "")
}

func TestGogenFunctionUndefinedThisNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @param {(x: number) => string} f */ function invoke(f) { return f(2); }
export class Probe { static run() { return invoke(function(x) { return (this === undefined) + ":" + x; }); } }`, true)
}

func TestGogenEpochABAPRefusal(t *testing.T) {
	gogenTSOracle(t, `class Probe { static run(): boolean { return Date.now() > 1700000000000; } }`, "clock.epochMilliseconds")
}

func TestGogenBigIntLocatedRefusalNodeOracle(t *testing.T) {
	out, err := exec.Command("node", "-e", `console.log(typeof BigInt("123456789012345678901234567890"))`).CombinedOutput()
	if err != nil || string(out) != "bigint\n" {
		t.Fatal(err, string(out))
	}
	_, diags := lowerProbe(t, sourceProbe(t, `export class Probe { static run() { return BigInt("123456789012345678901234567890"); } }`))
	for _, d := range diags {
		if d.Category == "unsupported-bigint" && strings.Contains(d.Loc, "input.ts:") && strings.Contains(d.Message, "no HIR representation") {
			return
		}
	}
	t.Fatal("missing located bigint refusal", diags)
}

func TestGogenTSFunctionParameterNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Probe {
 static invoke(place: (x: number, y: number) => number): number { return place(2, 3); }
 static filter(ours: (x: string) => boolean): string { return ["a", "b"].filter((x) => ours(x)).join(","); }
 static run(): string { return Probe.invoke((x,y) => x * 10 + y) + ":" + Probe.filter((x) => x === "a"); }
}`, "")
}

func TestGogenRecursiveRendererNodeOracle(t *testing.T) {
	gogenTSOracle(t, `type Expr = {e: "num", value: number, meta: {}} | {e: "wrap", child: Expr};
interface Renderers { readonly tag: string; render(x: Expr): string; }
class Probe {
 static invoke(render: (x: Expr) => string, e: Expr): string { return render(e); }
 static run(): string { const h: Renderers = {tag: "r", render: (x) => x.e === "num" ? "n" + x.value : "wrap"}; const {render} = h; let local: (x: Expr) => string = (x) => x.e === "num" ? "l" + x.value : "wrap"; return render({e:"num",value:2,meta:{}}) + Probe.invoke(render, {e:"num",value:3,meta:{}}) + local({e:"num",value:4,meta:{}}); }
}`, "union.")
}
