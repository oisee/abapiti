package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frontend-inventory.mjs: `const enumerate = core.Registry.prototype.getObjects`
// taken at module initialization and later called as enumerate.call(reg). The
// alias holds Reg's own method, so the call is C.prototype.m.call(recv).
func TestGogenA31PrototypeMethodAliasNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `interface IReg { getObjects(): Generator<string, void, undefined>; label(prefix: string, n: number): string }
class Reg implements IReg {
 items: string[] = [];
 tag = "r";
 *getObjects(): Generator<string, void, undefined> { for (const x of this.items) yield this.tag + x; }
 label(prefix: string, n: number): string { return prefix + this.tag + n; }
}
class Sub extends Reg { constructor() { super(); this.tag = "s"; } }
class Other implements IReg {
 *getObjects(): Generator<string, void, undefined> { yield "o"; }
 label(prefix: string, n: number): string { return prefix + "o" + n; }
}
const enumerate: Reg["getObjects"] = Reg.prototype.getObjects;
const label = Reg.prototype.label;
class Probe {
 static each(reg: IReg): string { return [...(reg instanceof Reg ? enumerate.call(reg) : reg.getObjects())].join(";"); }
 static run(): string {
  const r = new Reg(); r.items.push("a", "b");
  const s = new Sub(); s.items.push("c");
  return Probe.each(r) + "|" + Probe.each(s) + "|" + Probe.each(new Other()) + "|" + label.call(s, "p", 2);
 }
}`})
}

// The alias is only lowered as alias.call(receiver, ...): an unbound call has
// this = undefined in JavaScript and is refused at the use.
func TestGogenA31PrototypeMethodAliasUnboundRefused(t *testing.T) {
	dir := t.TempDir()
	src := `class Reg { tag = "r"; label(): string { return "R" + this.tag; } }
const label = Reg.prototype.label;
export class Probe { static run(): string { return label(); } }
`
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := LowerProjectWithOptions(filepath.Join(dir, "tsconfig.json"), []string{"probe.ts"}, nil, nil, LowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range r.Blocking {
		if strings.Contains(d.Message, "unbound prototype method label") && strings.HasPrefix(d.Loc, "probe.ts:3:") {
			return
		}
	}
	t.Fatalf("unbound alias call not refused at the use: %v %v", r.Blocking, r.Diagnostics)
}

// A source file of a separate package checkout outside the tsconfig directory
// (reached through tsconfig paths, like @abaplint/core in the gogen pilot) is
// named by its path inside the package, so registry overrides keyed by
// src/... apply and generated names do not depend on the checkout location.
func TestGogenA31PackageSourceNames(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("lint/packages/core/package.json", `{"name":"@x/core"}`)
	write("lint/packages/core/src/index.ts", `export {Box} from "./box";`+"\n")
	write("lint/packages/core/src/box.ts", `export class Box { v = 1; get(): number { return this.v; } }`+"\n")
	write("other/package.json", `{"name":"other"}`)
	write("other/src/box.ts", `export class Box2 { w = 2; }`+"\n")
	write("proj/tsconfig.json", `{"compilerOptions":{"strict":true,"target":"ES2022","baseUrl":".","paths":{"@x/core":["../lint/packages/core/src/index.ts"],"other":["../other/src/box.ts"]}},"include":["*.ts"]}`)
	write("proj/main.ts", `import {Box} from "@x/core"; import {Box2} from "other";
export class Probe { static run(): number { return new Box().get() + new Box2().w; } }
`)
	p, err := Load(filepath.Join(root, "proj", "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, name := range p.SourceFiles() {
		if strings.Contains(name, root) {
			got[p.sourceName(name)] = name
		}
	}
	for _, want := range []string{"main.ts", "src/index.ts"} {
		if got[want] == "" {
			t.Errorf("missing source name %s in %v", want, got)
		}
	}
	// src/box.ts names two packages' files: both keep their tsconfig-relative names.
	if got["src/box.ts"] != "" || got["../lint/packages/core/src/box.ts"] == "" || got["../other/src/box.ts"] == "" {
		t.Errorf("ambiguous package names were not kept apart: %v", got)
	}
	prog, diags, err := p.Lower([]string{"main.ts", "../lint/packages/core/src/index.ts", "../lint/packages/core/src/box.ts", "../other/src/box.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Errorf("lower: %s", d)
		}
	}
	names := []string{}
	for _, c := range prog.Classes {
		names = append(names, c.Name)
	}
	if !strings.Contains(strings.Join(names, " "), "../lint/packages/core/src/box.ts.Box") {
		t.Errorf("class names %v", names)
	}
}

// ir-children.ts / sink-locals.ts isIRArray: Array.isArray on a tagged value
// whose object constituents include intersections (Pick<T, K> & {...}). Only
// a value whose every object constituent is an array may be decided by the
// box's object tag; an intersection is an object that is not an array.
func TestGogenA31IsArrayIntersectionNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `type Leaf = {readonly e: "var"; readonly name: string};
type V = string | null | readonly V[] | (Pick<Leaf, "e"> & {readonly name?: string});
function isArr(v: V): v is readonly V[] { return Array.isArray(v); }
function count(v: V): number { if (isArr(v)) { let n = 0; for (const x of v) n += count(x); return n; } return v === null ? 0 : typeof v === "string" ? 1 : 10; }
class Probe {
 static run(): string {
  const leaf: V = {e: "var", name: "a"};
  const tree: V = ["s", leaf, [null, leaf, ["t"]]];
  return String(isArr(leaf)) + "," + isArr("x") + "," + isArr(null) + "," + isArr(tree) + "," + count(tree);
 }
}`})
}

// sink-locals.ts namesIn: a type predicate narrows a tagged Value to the
// recursive family Expression = Inspection<Expr> before anything else names
// that family; the discriminant and payload reads use the family layout.
func TestGogenA31NarrowedFamilyReadNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `type Expr = {readonly e: 'var'; readonly name: string; readonly ref?: boolean} | {readonly e: 'call'; readonly args: readonly Expr[]} | {readonly e: 'str'; readonly value: string};
type Stmt = {readonly s: 'clear'; readonly target: Expr} | {readonly s: 'assign'; readonly target: Expr; readonly value: Expr};
type IRNode = Expr | Stmt;
type IRValue = IRNode | readonly IRValue[] | string | number | boolean | null | undefined;
type InspectionKeys<T> = Extract<keyof T, 'e' | 's' | (T extends {readonly e: 'var'} ? 'name' : never)>;
type Inspection<T> = T extends readonly (infer V)[] ? readonly Inspection<V>[]
  : T extends object ? Pick<T, InspectionKeys<T>> & {
    readonly [K in Exclude<keyof T, InspectionKeys<T>>]?: Inspection<T[K]>;
  } : T;
type Value = Inspection<IRValue>;
type Expression = Inspection<Expr>;
function isIRArray(value: Value): value is readonly Value[] { return Array.isArray(value); }
function isExpr(value: Value): value is Expression { return !!value && typeof value === 'object' && 'e' in value && typeof value.e === 'string'; }
function namesIn(node: Value, names = new Set<string>()): Set<string> {
  if (!node || typeof node !== 'object') return names;
  if (isIRArray(node)) { for (const value of node) namesIn(value, names); return names; }
  if (isExpr(node) && node.e === 'var') names.add(node.name);
  if (isExpr(node) && node.e === 'call') names.add('call' + (node.args?.length ?? -1));
  return names;
}
class Probe {
 static run(): string {
  const body: readonly Value[] = [{e: 'var', name: 'a'}, [{e: 'call', args: [{e: 'var', name: 'b'}]}, [{e: 'var', name: 'c'}, 'x', 3]], {e: 'str'}, null, {e: 'call'}];
  return [...namesIn(body)].join(",") + "|" + namesIn(null).size + "|" + namesIn("s").size;
 }
}`})
}
