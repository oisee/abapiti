package tsfront

import (
	"strings"
	"testing"
)

// A member declared `x?: undefined` reads undefined like `x?: never`: a union
// with a sibling that stores the field joins to the sibling's optional
// (emit-go-types.ts SupportedAttribute | UnsupportedAttribute).
func TestGogenA25UndefinedMemberUnionNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Base { readonly name: string; readonly static?: boolean; readonly unsupported?: string }
export interface Supported extends Omit<Base, 'static'> { readonly static?: boolean; readonly unsupported?: undefined; readonly type: string }
export interface Unsupported extends Omit<Supported, 'type' | 'unsupported'> { readonly type?: string; readonly unsupported: string }
export type Attr = Supported | Unsupported;
`, `/** @typedef {import("./types.ts").Attr} Attr */
/** @param {readonly Attr[]} attrs @returns {string} */
const show = (attrs) => attrs.filter((a) => !a.unsupported).map((a) => a.name).join("/") + ":" + attrs.map((a) => String(a.unsupported)).join("/");
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Attr[]} */ const attrs = [{name: "A", type: "i"}, {name: "B", unsupported: "why"}, {name: "C", type: "s", unsupported: undefined}];
    return show(attrs);
  }
}
`, true, true)
}

// A plain member read on A | B | undefined (a JSDoc find() result in a .mjs
// without checkJs) narrows the receiver and reads the common field.
func TestGogenA25NullableUnionFieldNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Base { readonly name: string; readonly static?: boolean; readonly unsupported?: string; readonly type: {readonly k: string} }
export interface Supported extends Omit<Base, 'static'> { readonly static?: boolean; readonly unsupported?: undefined }
export interface Unsupported extends Omit<Supported, 'type' | 'unsupported'> { readonly type?: {readonly k: string}; readonly unsupported: string }
export type Attr = Supported | Unsupported;
`, `/** @typedef {import("./types.ts").Attr} Attr */
/** @param {readonly Attr[]} attrs @param {string} n @returns {Attr | undefined} */
const attr = (attrs, n) => attrs.find((x) => x.name === n && !x.unsupported);
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Attr[]} */ const attrs = [{name: "A", type: {k: "i"}}, {name: "B", unsupported: "why"}, {name: "C", type: {k: "s"}}];
    const a = attr(attrs, "C").type;
    return String(a?.k) + ":" + attr(attrs, "A").name + ":" + String(attr(attrs, "A").type?.k) + ":" + String(attr(attrs, "B")?.name);
  }
}
`, true, true)
}

// A function-valued parameter (with a default) passed by name to an array
// HOF: called as predicate(element, index, array) with this undefined.
func TestGogenA25NamedPredicateNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Obj { getName(): string }
`, `/** @typedef {import("./types.ts").Obj} Obj */
/** @param {string} n @returns {Obj} */
const named = (n) => ({getName: () => n});
/** @param {Obj[]} objs @param {(obj: Obj) => boolean} predicate @returns {Obj[]} */
function pick(objs, predicate = () => true) { return [...objs].filter(predicate); }
/** @param {Obj[]} objs @param {(obj: Obj, i: number, all: Obj[]) => boolean} predicate @returns {string[]} */
function names(objs, predicate) { return objs.filter(predicate).map((o) => o.getName()); }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Obj[]} */ const objs = [named("a"), named("bb"), named("c")];
    const long = pick(objs, (o) => o.getName().length > 1).map((o) => o.getName()).join("/");
    const all = pick(objs).length;
    const odd = names(objs, (o, i, a) => i % 2 === 0 && a.length === 3).join("/");
    return long + ":" + all + ":" + odd;
  }
}
`, true)
}

// Object.defineProperties with getters (frontend-inventory.mjs) has no exact
// property-descriptor ABI: a located refusal, not an unresolved identifier.
func TestGogenA25DefinePropertiesRefusal(t *testing.T) {
	for _, source := range []string{
		`class Probe {static run(){const s={raw:"x"};Object.defineProperties(s,{h:{get(){return this.raw+"!";}}});return s.raw;}}`,
		`class Probe {static run(){const s={raw:"x"};Object.defineProperty(s,"h",{value:1});return s.raw;}}`,
	} {
		_, ds := lowerProbe(t, sourceProbe(t, source))
		found := false
		for _, d := range ds {
			if d.Category == "unsupported-property-descriptor" && strings.Contains(d.Loc, "input.ts:") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no located property-descriptor refusal for %s: %v", source, ds)
		}
	}
}

// A recursive family whose literal payload admits bigint (sqlscript-types.ts
// SqlValue) keeps its tagged value slot instead of failing the family.
// ABAP: recursive families have no ABAP emitter yet (union.admit refusal).
func TestGogenA25BigIntFamilyMemberNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type SqlValue = string | number | bigint | boolean | null;
export type SqlExpr =
  | {node: "lit"; value: SqlValue}
  | {node: "bin"; op: string; left: SqlExpr; right: SqlExpr}
  | {node: "call"; fn: string; args: readonly SqlExpr[]};
`, `/** @typedef {import("./types.ts").SqlExpr} SqlExpr */
/** @param {import("./types.ts").SqlValue} value @returns {SqlExpr} */
function lit(value) { return {node: "lit", value}; }
/** @param {string} op @param {SqlExpr} left @param {SqlExpr} right @returns {SqlExpr} */
function bin(op, left, right) { return {node: "bin", op, left, right}; }
/** @param {SqlExpr} e @returns {string} */
function show(e) {
  if (e.node === "lit") return typeof e.value + ":" + String(e.value);
  if (e.node === "bin") return "(" + show(e.left) + " " + e.op + " " + show(e.right) + ")";
  return e.fn + "[" + e.args.map(show).join(",") + "]";
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {SqlExpr} */ const call = {node: "call", fn: "f", args: [lit(null), lit(true), lit("s")]};
    return show(bin("+", lit(12345678901234567890n), bin("*", lit(2), call)));
  }
}
`, false)
}

// A receiver lowered as a tagged value (Program | Class[] & Partial<Program>,
// emit-go-types.ts ProgramInput) whose checked member type is a Map or Set:
// the member read converts with a checked reference view before the call.
func TestGogenA25DynamicMapReceiverNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Cls { readonly name: string }
export interface Program { readonly classes: readonly Cls[]; readonly attrs?: ReadonlyMap<string, readonly string[]>; readonly structs: Map<string, number>; readonly seen?: Set<string> }
export type ClassList = Cls[] & Partial<Program>;
export type ProgramInput = Program | ClassList;
`, `/** @param {import("./types.ts").ProgramInput} program @param {string} k @returns {string} */
function show(program, k) {
  const attrs = (program.attrs?.get(k) ?? []).join("/");
  const has = program.structs?.has(k) ?? "none";
  const vals = [...(program.structs?.values() ?? [])].join("/");
  return attrs + ":" + String(has) + ":" + vals + ":" + String(program.seen?.has(k)) + ":" + (program.classes ?? []).map((c) => c.name).join("/");
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Program} */ const p = {classes: [{name: "A"}], attrs: new Map([["x", ["a", "b"]]]), structs: new Map([["x", 1], ["y", 2]]), seen: new Set(["x"])};
    /** @type {import("./types.ts").Program} */ const q = {classes: [], structs: new Map()};
    /** @type {import("./types.ts").ClassList} */ const list = [{name: "B"}];
    return [show(p, "x"), show(p, "z"), show(q, "x"), show(list, "x")].join(" ");
  }
}
`, true)
}

func TestGogenA25UnionFieldFamilyTypeNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `import type * as IR from "./types.ts";
interface Meta { readonly id?: string }
export type Ty = Meta & ({readonly k: "a"} | {readonly k: "t"; readonly row: Ty});
export interface Base { readonly name: string; readonly unsupported?: string; readonly type: IR.Ty }
export interface Supported extends Omit<Base, 'unsupported'> { readonly unsupported?: undefined }
export interface Unsupported extends Omit<Supported, 'type' | 'unsupported'> { readonly type?: IR.Ty; readonly unsupported: string }
export type Attr = Supported | Unsupported;
`, `/** @param {import("./types.ts").Ty | undefined} t @returns {string} */
const show = (t) => !t ? "-" : t.k === "t" ? "t(" + show(t.row) + ")" : t.k;
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Attr[]} */ const attrs = [{name: "A", type: {k: "t", row: {k: "a"}}}, {name: "B", unsupported: "why"}];
    return attrs.map((a) => a.name + show(a.type)).join(",");
  }
}
`, false, true)
}
