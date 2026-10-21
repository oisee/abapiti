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

func gogenOracle(t *testing.T, source, expected string, abapRefusal ...string) {
	t.Helper()
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	gogenCheck(t, p, "probe.ts", source, expected, abapRefusal...)
}

func gogenCheck(t *testing.T, p *hir.Program, file, source, expected string, abapRefusal ...string) {
	t.Helper()
	if t.Failed() {
		return
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry(file+".Probe", "run")))
	if got != expected+"\n" {
		t.Fatalf("Go %q want %q", got, expected)
	}
	_, abapErr := emitABAP(t, p)
	if len(abapRefusal) > 0 {
		if abapErr == nil || !strings.Contains(abapErr.Error(), abapRefusal[0]) || !strings.Contains(abapErr.Error(), file+":") {
			t.Fatalf("expected located ABAP refusal %q, got %v", abapRefusal[0], abapErr)
		}
	} else if abapErr != nil {
		t.Fatal(abapErr)
	}
	if _, err := exec.LookPath("node"); err == nil {
		want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(want))
		}
		if got != string(want) {
			t.Fatalf("Go %q Node %q", got, want)
		}
	} else {
		t.Log("Node absent; Go and ABAP emission/refusal checks still ran")
	}
}

func TestGogenJSDocObjectParameters(t *testing.T) {
	source := `/** @typedef {{expr: string, place: string}} Renderers */
 /** @param {number} st @param {number} ctx @param {number} t @param {Renderers} renderers @returns {string} */
 function f(st, ctx, t, {expr, place}) {return st + ctx + t + expr + place;}
 /** @param {string} program @param {{goType:string, ident:string, classes:string, poly:string, ancestorsOf:string}} helpers @returns {string} */
 function g(program, {goType, ident, classes: CLASSES, poly: POLY, ancestorsOf}) {return program + goType + ident + CLASSES + POLY + ancestorsOf;}
 class Probe {/** @returns {string} */ static run() {return f(1,2,3,{expr:"e",place:"p"})+":"+g("p",{goType:"g",ident:"i",classes:"C",poly:"P",ancestorsOf:"a"});}}`
	dir := t.TempDir()
	for file, content := range map[string]string{
		"probe.js":      source,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","allowJs":true,"checkJs":true,"noEmit":true},"files":["probe.js"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	project, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, diags, err := project.Lower([]string{"probe.js"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Error(d)
		}
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	gogenCheck(t, p, "probe.js", source, "6ep:pgiCPa")
}

func TestGogenObjectParameters(t *testing.T) {
	gogenOracle(t, `interface ByteRenderers {expr: string; place: string; rowValue: number;}
 function f(st: string, ctx: string, t: number, {expr, place, rowValue}: ByteRenderers): string {
  expr = expr + "!";
  return st + ctx + t + expr + place + rowValue;
 }
 function g(program: string, {classes: CLASSES, poly: POLY}: {classes: string; poly: string}): string { return program + CLASSES + POLY; }
 class Probe { static run(): string { const r = {expr:"e", place:"p", rowValue:3}; return f("s","c",2,r) + ":" + r.expr + ":" + g("g",{classes:"C",poly:"P"}); } }`, "sc2e!p3:e:gCP")
}

func TestGogenObjectParameterGetters(t *testing.T) {
	gogenOracle(t, `class Source {
 static trace="";
 get expr(): string {Source.trace += "e";return Source.trace;}
 get place(): string {Source.trace += "p";return Source.trace;}
 static arg(): number {Source.trace += "a";return 1;}
 }
 function read({expr, place: renamed}: Source, n: number): string {return expr+":"+renamed+":"+Source.trace+n;}
 class Probe {static run(): string {return read(new Source(),Source.arg());}}`, "ae:aep:aep1")
}

func TestGogenObjectParameterRefusals(t *testing.T) {
	for _, pattern := range []string{"{...rest}", "{a: {b}}"} {
		t.Run(pattern, func(t *testing.T) {
			requireDiagnostic(t, sourceProbe(t, "function f("+pattern+": any):void {}"), "unsupported-param")
		})
	}
}

func TestGogenObjectParameterNameCollision(t *testing.T) {
	gogenOracle(t, `function f($binding1:string, {expr}: {expr:string}):string {return $binding1+expr;}
 class Probe {static run():string {return f("a",{expr:"b"});}}`, "ab")
}

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

func TestGogenFunctionReceiverRefusalNodeOracle(t *testing.T) {
	testGogenThisRefusal(t, `class Probe { static run() { const h = {f: function() { return this === undefined; }}; const f = h.f; return h.f() + ":" + f(); } }`, "false:true\n")
	testGogenThisRefusal(t, `class Probe { static run() { const h = {f: function() { return this === h; }}; const f = h.f; return h.f() + ":" + f(); } }`, "true:false\n")
	testGogenThisRefusal(t, `class Probe { static run() { const f = function() { return this === undefined; }; const h = {f}; const g = h.f; return h.f() + ":" + g(); } }`, "false:true\n")
	testGogenThisRefusal(t, `class Probe { static run() { function f() { return this === undefined; } const h = {f}; const g = h.f; return h.f() + ":" + g(); } }`, "false:true\n")
	testGogenThisRefusal(t, `function invoke(f) { return f(2); } class Probe { static run() { return invoke(function(x) { return (this === undefined) + ":" + x; }); } }`, "true:2\n")
}

func testGogenThisRefusal(t *testing.T, source, want string) {
	t.Helper()
	if got := conversionOracle(t, source); got != want {
		t.Fatalf("Node %q, want %q", got, want)
	}
	_, diags := lowerProbe(t, sourceProbe(t, source))
	for _, d := range diags {
		if d.Category == "unsupported-closure" && strings.Contains(d.Loc, "input.ts:") && strings.Contains(d.Message, "call-site this") {
			return
		}
	}
	t.Fatal("missing located call-site this refusal", diags)
}

func TestGogenMapNumberNodeOracle(t *testing.T) {
	source := `type M = Map<number,string>; class Probe { static run(): string { const m:M = new Map(); const n = parseFloat("x"); m.set(n,"a"); m.set(n,"b"); return m.has(n) + ":" + m.size + ":" + m.get(n) + ":" + m.delete(n) + ":" + m.has(n); } }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestGogenEpochABAPRefusal(t *testing.T) {
	gogenTSOracle(t, `class Probe { static run(): boolean { return Date.now() > 1700000000000; } }`, "clock.epochMilliseconds")
}

func TestGogenBigIntLocatedRefusalNodeOracle(t *testing.T) {
	out, err := exec.Command("node", "-e", `console.log(typeof BigInt(123))`).CombinedOutput()
	if err != nil || string(out) != "bigint\n" {
		t.Fatal(err, string(out))
	}
	_, diags := lowerProbe(t, sourceProbe(t, `export class Probe { static run() { return BigInt(123); } }`))
	for _, d := range diags {
		if d.Category == "unsupported-bigint" && strings.Contains(d.Loc, "input.ts:") && strings.Contains(d.Message, "decimal string") {
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
