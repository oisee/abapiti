package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gohir "github.com/oisee/abapiti/hir/golang"
)

// a19Check compares Go with Node for a probe class and requires ABAP to
// either emit or refuse at a located probe.ts position.
func a19Check(t *testing.T, files map[string]string) {
	t.Helper()
	var all strings.Builder
	for _, name := range []string{"lib.ts", "probe.ts"} {
		if src, ok := files[name]; ok {
			all.WriteString(strings.ReplaceAll(src, `import {`, `// import {`) + "\n")
		}
	}
	want := nodePlatformOracle(t, strings.ReplaceAll(all.String(), "export ", ""))
	var roots []string
	for name := range files {
		roots = append(roots, name)
	}
	p := lowerStatementsProbe(t, files, roots)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	if _, err := emitABAP(t, lowerStatementsProbe(t, files, roots)); err != nil && !strings.Contains(err.Error(), ".ts:") {
		t.Fatalf("wanted ABAP or a located refusal, got %v", err)
	}
}

func TestGogenA19MapEntryDestructuringNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `interface Use { path: string[]; first: Map<string, number> }
class Probe {
 static run(): string {
  const uses = new Map<string, Use>();
  uses.set("a", {path: ["x", "y"], first: new Map<string, number>([["x", 1]])});
  uses.set("b", {path: [], first: new Map<string, number>()});
  uses.set("a", {path: ["z", "w", "v"], first: new Map<string, number>([["w", 2]])});
  const out: string[] = [];
  for (const [name, use] of uses) {
   out.push(name + use.path.length);
   for (let i = use.path.length - 1; i > 0; i--) { const body = use.path[i]; if (!use.first.has(body)) continue; out.push(body + use.first.get(body)!); break; }
  }
  const stores = new Map<string, string>([["k", "v"], ["k2", "v2"]]);
  for (const [owner, store] of stores) out.push(owner + "=" + store);
  const live = new Map<string, number>([["a", 1], ["b", 2]]);
  live.set("c", 3);
  for (const [k, v] of live) { out.push(k + v); if (k === "a") { live.delete("b"); live.set("c", 4); live.set("a", 9); } }
  for (const [k, v] of live) { out.push(k + v); live.delete(k); }
  out.push(String(live.size));
  live.set("x", 5).set("y", 6).set("z", 7);
  for (const [k] of live) { if (k === "x") { live.delete("x"); live.delete("z"); } out.push(k); }
  let n = 0;
  for (const [, v] of live) n += v;
  for (let [k] of live) { k = k + "!"; out.push(k); }
  out.push(String(n));
  return out.join(",");
 }
}`})
}

func TestGogenA19ArrayDestructuringReassignNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `class Node2 { text: string; constructor(text: string) { this.text = text; } }
class Probe {
 static calls = 0;
 static parts(n: number): readonly Node2[] { Probe.calls++; const r: Node2[] = []; for (let i = 0; i < n; i++) r.push(new Node2("p" + i)); return r; }
 static run(): string {
  const out: string[] = [];
  for (const n of [2, 1, 3]) {
   const [pat, subj] = Probe.parts(n);
   const untyped: any = Probe.parts(n + 1);
   const [first, second, third] = untyped;
   const a: string = first.text, b: string = second.text;
   let label = a + b;
   if (third === undefined) label = label + "-"; else { const base = label; const c: string = third.text; label = base + c; }
   out.push(label);
   let pattern = pat.text;
   if (n > 1) { const base = pattern; pattern = base + "!" + subj.text; }
   else pattern = pattern + "?" + String(subj === undefined);
   out.push(pattern);
  }
  out.push(String(Probe.calls));
  return out.join(",");
 }
}`})
}

func TestGogenA19OptionalNumberUpdateNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `interface Ctx { name: string; loop?: number }
class Probe {
 static calls = 0;
 static get(c: Ctx): Ctx { Probe.calls++; return c; }
 static run(): string {
  const out: string[] = [];
  const a: Ctx = {name: "a", loop: 0};
  const n = a.loop++;
  out.push("r" + n, "v" + a.loop);
  const m = Probe.get(a).loop++;
  out.push("r" + m, "v" + a.loop, "c" + Probe.calls);
  const p = ++a.loop;
  out.push("r" + p);
  a.loop--;
  out.push("v" + a.loop);
  const b: Ctx = {name: "b"};
  const q = b.loop++;
  out.push("nan" + String(q !== q) + String(b.loop !== b.loop) + String(b.loop === undefined));
  a.loop += 2;
  Probe.get(a).loop -= 1;
  b.loop = undefined;
  b.loop += 1;
  out.push("v" + a.loop, "nan" + String(b.loop !== b.loop), "c" + Probe.calls);
  const d: Ctx = {name: "d"};
  d.loop *= 3;
  out.push("nan" + String(d.loop !== d.loop));
  d.loop = 2;
  d.loop *= 3;
  out.push("v" + d.loop);
  const c: Ctx = {name: "c"};
  const r = --c.loop;
  out.push("nan" + String(r !== r) + String(c.loop !== c.loop));
  return out.join(",");
 }
}`})
}

// Sibling blocks reuse one local arrow name (frontend.mjs hasKeywords): each
// call binds the arrow declared in its own block, with that block's captures.
func TestGogenA19RestClosureCaptureNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `class Probe {
 static check(kind: number, text: string): string {
  if (kind === 1) {
   const keywords = text.split(" ");
   const hasKeywords = (...words: string[]) => keywords.some((_, i) => words.every((word, j) => keywords[i + j] === word));
   return "k" + String(hasKeywords("A", "B")) + String(hasKeywords("Z"));
  }
  if (kind === 2) {
   const words = text.split(" ").map((w) => w.toUpperCase());
   const hasKeywords = (...seq: string[]) => words.some((_, i) => seq.every((w, j) => words[i + j] === w));
   return "w" + String(hasKeywords("IGNORING", "CASE")) + String(hasKeywords()) + String(hasKeywords("X"));
  }
  return "none";
 }
 static run(): string {
  return [Probe.check(1, "A B C"), Probe.check(1, "B A"), Probe.check(2, "find x ignoring case"), Probe.check(2, "ignoring"), Probe.check(3, "")].join(",");
 }
}`})
}

// A nested block's arrow shadows the outer one only until the block ends.
func TestGogenA19ShadowedLocalArrowNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `class Probe {
 static check(text: string): string {
  const out: string[] = [];
  const f = (s: string) => "outer" + s;
  out.push(f("1"));
  {
   const words = text.split(" ");
   const f = (s: string) => "inner" + s + words.length;
   out.push(f("2"));
  }
  out.push(f("3"));
  for (const w of ["a", "b"]) { const f = (s: string) => w + s; out.push(f("4")); }
  out.push(f("5"));
  return out.join(",");
 }
 static run(): string { return Probe.check("x y z"); }
}`})
}

func TestGogenA19ImportedFunctionValueNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{
		"lib.ts": `export function source(n: number, extra?: string): string { return "s" + n + (extra ?? ""); }
export function sourceOperand(n: number): string { return "o" + n; }
export const C = (n: number): string => "c" + n;
`,
		"probe.ts": `import {source, sourceOperand, C} from "./lib.ts";
const helpers = {source, sourceOperand, convert: C};
class Probe {
 static use(deps: {source: (n: number) => string, sourceOperand: (n: number) => string}): string { return deps.source(1) + deps.sourceOperand(2); }
 static run(): string {
  const local = {source, C};
  return [Probe.use({sourceOperand, source}), helpers.source(3), helpers.convert(4), local.C(5), local.source(6, "x")].join(",");
 }
}`})
}

const a19TypeUnion = `interface Meta { readonly qname?: string }
export type Type = Meta & (
 | { readonly k: "i" }
 | { readonly k: "string" }
 | { readonly k: "xstring" }
 | { readonly k: "exc" }
 | { readonly k: "c"; readonly len: number }
 | { readonly k: "x"; readonly len: number }
 | { readonly k: "table"; readonly row: Type });
const I: Extract<Type, {k: "i"}> = {k: "i"};
const S: Extract<Type, {k: "string"}> = {k: "string"};
const XS: Extract<Type, {k: "xstring"}> = {k: "xstring"};
const EXC: Extract<Type, {k: "exc"}> = {k: "exc"};
const C3: Extract<Type, {k: "c"}> = {k: "c", len: 3};
const X4: Extract<Type, {k: "x"}> = {k: "x", len: 4};
function show(t: Type): string {
 switch (t.k) {
 case "c": case "x": return t.k + t.len;
 case "table": return "table(" + show(t.row) + ")";
 default: return t.k;
 }
}
`

func TestGogenA19ConditionalPayloadJoinNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": a19TypeUnion + `class Probe {
 static calls = 0;
 static typeOf(n: number): Type { Probe.calls++; return n > 1 ? {k: "table", row: I} : S; }
 static attr(exc: boolean, n: number): {name: string, type: Type} {
  const type = exc ? EXC : Probe.typeOf(n);
  return {name: "a" + n, type};
 }
 static substr(k: string): Type {
  const type = k === "string" ? S : k === "xstring" ? XS : k === "c" ? C3 : X4;
  return type;
 }
 static leaves(ts: Type[], outer: Type | undefined): string {
  const target = outer === undefined ? I : outer;
  const leaves = ts.map((t) => (["x", "xstring"].includes(t.k) ? I : t));
  return leaves.map(show).join("/") + "|" + (target === undefined ? "none" : show(target));
 }
 static run(): string {
  const out: string[] = [];
  out.push(show(Probe.attr(true, 2).type), show(Probe.attr(false, 2).type), show(Probe.attr(false, 0).type), String(Probe.calls));
  for (const k of ["string", "xstring", "c", "x"]) out.push(show(Probe.substr(k)));
  out.push(Probe.leaves([S, XS, X4, C3, {k: "table", row: XS}], X4), Probe.leaves([], undefined), Probe.leaves([I], S));
  return out.join(",");
 }
}`})
}

func TestGogenA19ConditionalSubsetContextNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": a19TypeUnion + `class Probe {
 static leaves(outer: Type | undefined): string {
  const target = outer?.k === "x" ? I : outer;
  const charTarget = target !== undefined && (target.k === "c" || target.k === "string");
  return (target === undefined ? "none" : show(target)) + String(charTarget);
 }
 static run(): string {
  return [Probe.leaves(undefined), Probe.leaves(X4), Probe.leaves(C3), Probe.leaves(I), Probe.leaves({k: "table", row: X4})].join(",");
 }
}`})
}

// a19Trap runs Probe.run in Go and requires a located trap.
func a19Trap(t *testing.T, source, want string) {
	t.Helper()
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, err := gohir.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files["go.mod"] = "module probe\n\ngo 1.26.0\n"
	files["main.go"] = "package main\nimport \"fmt\"\nfunc main(){ fmt.Println(" + goEntry("probe.ts.Probe", "run") + "()) }\n"
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), want) {
		t.Fatalf("wanted a trap %q, got %v\n%s", want, err, out)
	}
}

// An entry appended during the loop would be visited by JavaScript after the
// snapshot ends: Go traps at the next visit instead of skipping it.
func TestGogenA19MapForOfAppendTraps(t *testing.T) {
	for _, body := range []string{
		`if (k === "a") m.set("c", 3);`,
		`if (k === "b") { m.delete("a"); m.set("a", 0); }`,
		`if (k === "b") { m.delete("b"); m.set("b", 0); }`,
	} {
		a19Trap(t, `class Probe {
 static run(): string {
  const m = new Map<string, number>([["a", 1], ["b", 2]]);
  const out: string[] = [];
  for (const [k, v] of m) { out.push(k + v); `+body+` }
  return out.join(",");
 }
}`, "probe.ts:5:3: Map for-of appended an entry during iteration")
	}
}

// JavaScript would iterate a string's code points; the checked view traps.
func TestGogenA19UntypedDestructuringNonArrayTraps(t *testing.T) {
	a19Trap(t, `class Probe {
 static run(): string {
  const untyped: any = "xy";
  const [a, b] = untyped;
  const s: string = a, u: string = b;
  return s + u;
 }
}`, "live typed array requires array storage")
}
