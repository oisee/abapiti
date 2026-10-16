package tsfront

import "testing"

// A `Condition | null` local narrowed by truthiness joins with a family
// literal in a conditional (frontend.mjs stringComparison/compareBytes), and
// a boxed union value narrowed into an optional family (sink-locals.ts).
// Families have no ABAP ABI yet (union.* ops refuse there): Go only.
func TestGogenA27NullableFamilyJoinNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Cond = {readonly c: "eq"; readonly l: string; readonly r: string} | {readonly c: "not"; readonly x: Cond} | {readonly c: "and"; readonly xs: readonly Cond[]};
`, `/** @typedef {import("./types.ts").Cond} Cond */
/** @param {string} op @param {string} l @param {string} r @returns {Cond | null} */ function cmp(op, l, r) { return op === "=" ? {c: "eq", l, r} : null; }
/** @param {Cond} c @returns {string} */ function show(c) { return c.c === "eq" ? c.l + "=" + c.r : c.c === "not" ? "!(" + show(c.x) + ")" : c.xs.map(show).join("&"); }
/** @param {string} op @param {boolean} not @returns {Cond} */ function cond(op, not) {
  const sc = ["=", "<"].includes(op) ? cmp(op, "a", "b") : null;
  if (sc) return not ? {c: "not", x: sc} : sc;
  const r2 = cmp(op === "<" ? "=" : op, "c", "d");
  if (r2) return not ? {c: "not", x: r2} : r2;
  return {c: "and", xs: []};
}
/** @param {Cond | string | null} v @returns {string} */ function pick(v) {
  const c = typeof v === "object" && v !== null ? v : undefined;
  return c ? show(c) : "none:" + String(v);
}
export class Probe {
  /** @returns {string} */ static run() {
    return [cond("=", false), cond("=", true), cond("<", true), cond("?", false)].map(show).join(" ") + "|" + [pick(cond("=", true)), pick("s"), pick(null)].join(" ");
  }
}
`, false, true)
}

// Set.prototype.clear (frontend-owned.ts) empties the set in place: every
// alias observes it, size is 0, membership is gone, and later adds restart
// insertion order.
func TestGogenA27SetClearNodeOracle(t *testing.T) {
	runMjsOracle(t, `export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Set<string>} */ const s = new Set(["b", "a", "c"]);
    const alias = s;
    const before = s.size;
    s.clear();
    s.add("z");
    s.add("b");
    /** @type {Set<number>} */ const n = new Set([1, 2, 3]);
    for (const v of [1, 2]) { if (v === 2) n.clear(); }
    n.clear();
    n.add(7);
    return before + ":" + alias.size + ":" + alias.has("a") + ":" + [...alias].join(",") + ":" + [...n].join(",") + ":" + n.has(1);
  }
}
`, true)
}

// Object literal method shorthand without this/super/arguments is a function
// value capturing its scope (frontend-owned.ts returns {declarations,
// unescaped(p, ctx) {...}, width(...) {...}}).
func TestGogenA27ObjectLiteralMethodNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Ownership { readonly declarations: Set<string>; unescaped(p: string | undefined, depth: number): boolean | undefined; width(p: string): number }
export function make(names: readonly string[]): Ownership {
  const declarations = new Set(names);
  let calls = 0;
  return {declarations, unescaped(p: string | undefined, depth: number): boolean | undefined {
    calls++;
    if (p === undefined) return undefined;
    return declarations.has(p) && depth < calls;
  }, width(p: string): number { return p.length + calls; }};
}
`, `import {make} from "./types.ts";
export class Probe {
  /** @returns {string} */ static run() {
    const o = make(["a", "bb"]);
    const r = [o.unescaped("a", 0), o.unescaped(undefined, 0), o.unescaped("bb", 5), o.unescaped("zz", 0)].map((x) => String(x)).join(",");
    o.declarations.add("zz");
    return r + ":" + o.unescaped("zz", 0) + ":" + o.width("abc") + ":" + o.declarations.size;
  }
}
`, true, true)
}

// `obj instanceof C` on a generic T (frontend-inventory.mjs
// rttiClassInventory) narrows to T & C: calls use C's own methods.
func TestGogenA27GenericInstanceofNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IObject { getName(): string }
export class Def { readonly sup: string | undefined; constructor(sup: string | undefined) { this.sup = sup; } getSuperClass(): string | undefined { return this.sup; } }
export class Clas implements IObject { readonly n: string; readonly d: Def | undefined; constructor(n: string, d: Def | undefined) { this.n = n; this.d = d; } getName(): string { return this.n; } getDefinition(): Def | undefined { return this.d; } }
export class Intf implements IObject { readonly n: string; constructor(n: string) { this.n = n; } getName(): string { return this.n; } }
`, `import {Clas, Intf, Def} from "./types.ts";
import * as core from "./types.ts";
/** @template {import("./types.ts").IObject} T @param {Iterable<T>} objects @param {Map<string, string | null>} supers @returns {Generator<T>} */
function* inventory(objects, supers) {
  for (const obj of objects) {
    if (obj instanceof core.Clas) supers.set(obj.getName().toUpperCase(), obj.getDefinition()?.getSuperClass()?.toUpperCase() ?? null);
    yield obj;
  }
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Map<string, string | null>} */ const supers = new Map();
    const names = [...inventory([new Clas("a", new Def("zb")), new Intf("i"), new Clas("c", undefined)], supers)].map((o) => o.getName());
    return names.join(",") + ":" + [...supers].map(([k, v]) => k + "=" + v).join(",");
  }
}
`, true, true)
}
