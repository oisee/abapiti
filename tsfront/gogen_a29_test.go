package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// util.isDeepStrictEqual (emit-table-move.ts) on recursive IR families:
// SameValue primitives (NaN equal, -0 not +0), own-key presence (absent vs
// present undefined), key order independence, null vs undefined, payload
// kinds, nested arrays and plain values. Go only; ABAP refuses at the call.
func TestGogenA29DeepStrictEqualNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type T = {readonly k: 'int'} | {readonly k: 'table'; readonly row: T};
export type E = {readonly e: 'var'; readonly name: string; readonly type?: T}
  | {readonly e: 'field'; readonly base: E; readonly name: string}
  | {readonly e: 'call'; readonly args: readonly E[]; readonly sep: E | null}
  | {readonly e: 'int' | 'float'; readonly value: number; readonly type?: T};
`, `import {isDeepStrictEqual} from "node:util";
import * as util from "node:util";
/** @typedef {import("./types.ts").E} E */
/** @param {E} a @param {E} b @returns {string} */ function eq(a, b) { return isDeepStrictEqual(a, b) ? "1" : "0"; }
/** @param {string} n @returns {E} */ function v(n) { return {e: "var", name: n}; }
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {E} */ const f1 = {e: "field", base: v("x"), name: "y"};
    /** @type {E} */ const f2 = {e: "field", base: {e: "var", name: "x"}, name: "y"};
    /** @type {E} */ const f3 = {e: "field", base: v("z"), name: "y"};
    /** @type {E} */ const u = {e: "var", name: "x", type: undefined};
    /** @type {E} */ const t1 = {e: "var", name: "x", type: {k: "table", row: {k: "int"}}};
    /** @type {E} */ const t2 = {name: "x", type: {k: "table", row: {k: "int"}}, e: "var"};
    /** @type {E} */ const n1 = {e: "float", value: NaN};
    /** @type {E} */ const n2 = {e: "float", value: Number("x")};
    /** @type {E} */ const z1 = {e: "float", value: 0};
    /** @type {E} */ const z2 = {e: "float", value: -0};
    /** @type {E} */ const i1 = {e: "int", value: 1};
    /** @type {E} */ const c1 = {e: "call", args: [v("a"), f1], sep: null};
    /** @type {E} */ const c2 = {e: "call", args: [v("a"), f2], sep: null};
    /** @type {E} */ const c3 = {e: "call", args: [v("a")], sep: null};
    /** @type {E} */ const c4 = {e: "call", args: [v("a"), f1], sep: v("s")};
    const xs = [eq(f1, f2), eq(f1, f3), eq(f1, f1), eq(v("x"), u), eq(u, u), eq(t1, t2), eq(t1, v("x")), eq(n1, n2), eq(z1, z2), eq(z1, z1),
      eq(i1, {e: "float", value: 1}), eq(c1, c2), eq(c1, c3), eq(c1, c4), eq(c4, {e: "call", args: [v("a"), f2], sep: v("s")}), eq(f1, v("y"))];
    /** @type {(number | undefined)[]} */ const h1 = [1, 9, 2];
    /** @type {(number | undefined)[]} */ const h2 = [1, undefined, 2];
    /** @type {(number | undefined)[]} */ const h3 = [1, 8, 2];
    delete h1[1];
    delete h3[1];
    const ps = [util.isDeepStrictEqual("a", "a"), isDeepStrictEqual(1, 1), isDeepStrictEqual(0, -0), isDeepStrictEqual(NaN, NaN), isDeepStrictEqual(null, undefined),
      isDeepStrictEqual(null, null), isDeepStrictEqual("1", 1), isDeepStrictEqual([1, 2], [1, 2]), isDeepStrictEqual(["a"], ["a", "b"]), isDeepStrictEqual(["a"], "a"), isDeepStrictEqual(h1, h2), isDeepStrictEqual(h1, h3)];
    return xs.join("") + ":" + ps.map((p) => p ? "1" : "0").join("");
  }
}
`, false, true)
}

// frontend-types.ts OwnershipSource: an interface extending a method
// interface and a mapped base (Partial<Pick<C, K>>). The mapped base carries
// no brand; the declaration lowers and the base interface's methods dispatch.
func TestGogenA29MappedInterfaceBaseNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IObject { getName(): string }
export class Obj implements IObject { readonly n: string; constructor(n: string) { this.n = n; } getName(): string { return this.n; } files(): string[] { return [this.n]; } }
export interface Source extends IObject, Partial<Pick<Obj, 'files'>> {}
export function name(s: Source): string { return s.getName() + "!"; }
`, `import {Obj, name} from "./types.ts";
export class Probe {
  /** @returns {string} */ static run() { return name(new Obj("a")) + name(new Obj("bc")); }
}
`, false, true)
}

// path.join (host-io.ts join(...parts)) against Node's posix.join on a
// differential grid: empty segments, ".", "..", above-root, absolute and
// trailing separators, repeated slashes; and the plain-argument form.
func TestGogenA29PathJoinNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `import * as path from "node:path";
export function join(...parts: string[]): string { return path.join(...parts); }
`, `import {join} from "./types.ts";
import {join as pj} from "node:path";
import * as path from "node:path";
export class Probe {
  /** @returns {string} */ static run() {
    const segs = ["", ".", "..", "a", "b/", "/c", "../d", "e/..", "./f/.", "//g//", "h/../..", "...", ".x", "x.", "/"];
    /** @type {string[]} */ const out = [join()];
    for (const a of segs) {
      out.push(join(a));
      for (const b of segs) {
        out.push(join(a, b));
        for (const c of ["..", "z/", "", "/w/../.."]) out.push(join(a, b, c));
      }
    }
    out.push(pj("a", "b", "../../.."), path.join("/a/", "..", "..", "b"), path.join("é", "../ü/x"));
    return out.join("|");
  }
}
`, false, true)
}

// The same comparison in a program without sparse-array runtime support.
func TestGogenA29DeepStrictEqualDenseNodeOracle(t *testing.T) {
	runMjsOracle(t, `import {isDeepStrictEqual} from "node:util";
export class Probe {
  /** @returns {string} */ static run() {
    const a = [["x", "y"], ["z"]], b = [["x", "y"], ["z"]], c = [["x"], ["y", "z"]];
    return [isDeepStrictEqual(a, b), isDeepStrictEqual(a, c), isDeepStrictEqual([0], [-0]), isDeepStrictEqual([NaN], [Number("q")]), isDeepStrictEqual({p: 1, q: [2]}, {q: [2], p: 1}), isDeepStrictEqual({p: 1}, {p: 1, q: undefined})].map((x) => x ? "1" : "0").join(",");
  }
}
`, false)
}

// ABAP has no ABI for either operation: located refusal at the call.
func TestGogenA29NodePlatformABAPRefusal(t *testing.T) {
	for _, src := range []string{
		`import {isDeepStrictEqual} from "node:util";
export class Probe { /** @returns {boolean} */ static run() { return isDeepStrictEqual(["a"], ["a"]); } }
`, `import * as path from "node:path";
export class Probe { /** @returns {string} */ static run() { return path.join("a", "..", "b"); } }
`} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(src), 0600)
		os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","noEmit":true,"allowJs":true},"include":["*.mjs"]}`), 0600)
		p, err := Load(filepath.Join(dir, "tsconfig.json"))
		if err != nil {
			t.Fatal(err)
		}
		prog, diags, err := p.Lower([]string{"probe.mjs"})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range diags {
			if !strings.HasPrefix(d.Category, "note-") {
				t.Fatalf("lower: %s", d)
			}
		}
		if _, err := emitABAP(t, prog); err == nil || !strings.Contains(err.Error(), "probe.mjs:") {
			t.Fatalf("wanted located ABAP refusal, got %v", err)
		}
	}
}
