package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frontend.mjs passes require-bound namespaces (abaplint, Statements, Nodes)
// in helper records; the callee reads class and enum members through them.
func TestGogenA22NamespaceValueNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {createRequire} from "node:module";
import {describe, same} from "./use.ts";
const require = createRequire(import.meta.url);
const transpilerRequire = createRequire(require.resolve("@fake/transpiler/package.json"));
/** @type {typeof import("@fake/core")} */
const core = transpilerRequire("@fake/core");
const {Kinds, Token} = core;

export class Probe {
  /** @returns {string} */
  static run() {
    const a = Kinds.CharKind.of(3);
    const b = new core.Kinds.IntKind();
    return describe(a, {Kinds, core, tag: "x"}) + "|" + describe(b, {Kinds, core, tag: "y"}) + "|" + describe(new Token("t"), {Kinds, core, tag: "z"}) + "|" + same({Kinds, core, tag: "a"}, {Kinds: core.Kinds, core, tag: "b"});
  }
}
`, "c3x|iy|ozH3function|true,true,true\n", map[string]string{"use.ts": `import type * as fake from "@fake/core";
interface H { readonly Kinds: typeof fake.Kinds; readonly core: typeof fake; readonly tag: string }
export function describe(k: object, h: H): string {
  const {Kinds, tag} = h;
  const {Token} = h.core;
  if (k instanceof Kinds.CharKind) return "c" + k.length + tag;
  if (k instanceof h.core.Kinds.IntKind) return "i" + tag;
  if (k instanceof Token) return "o" + tag + h.core.Level.High + h.core.LIMIT + typeof h.core.twice;
  return "?";
}
export function same(a: H, b: H): string {
  return [a.Kinds === b.Kinds, a.core === b.core, a.core.Kinds === b.Kinds].join(",");
}
`})
}

// frontend-inventory.mjs / frontend-provenance.mjs: the requirer is called
// directly, createRequire(base)("<spec>"), with the same JSDoc contract as
// the two-step form.
func TestGogenA22DirectRequireNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {createRequire} from "node:module";
const require = createRequire(import.meta.url);
/** @type {typeof import("@fake/core")} */
export const core = createRequire(require.resolve("@fake/transpiler/package.json"))("@fake/core");
/** @type {typeof import("@fake/transpiler/build/src/rearranger")} */
const {Rearranger} = createRequire(import.meta.url)("@fake/transpiler/build/src/rearranger");
const {Kinds} = core;

export class Probe {
  /** @returns {string} */
  static run() {
    const a = Kinds.CharKind.of(2);
    const b = new core.Kinds.IntKind();
    return a.name() + "|" + (b instanceof core.Kinds.IntKind) + "|" + new core.Token("q").text() + "|" + new Rearranger().run([a, b]);
  }
}
`, "C2|true|'q'|I>C2\n")
}

// frontend-inventory.mjs prepareCdsViews: a function requires lazily, typed
// Pick<typeof import(...), ...>, and calls the destructured functions.
func TestGogenA22LazyPickRequireNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {createRequire} from "node:module";
const require = createRequire(import.meta.url);

export class Probe {
  /** @returns {string} */
  static run() {
    /** @type {Pick<typeof import("@fake/core"), "twice" | "Token">} */ const {twice, Token} = require("@fake/core");
    return twice(4) + "|" + new Token("p").text();
  }
}
`, "8|'p'\n")
}

// frontend.mjs origin(): `const found = all && all.find(...)` with all an
// optional array; the result is undefined when all is.
func TestGogenA22OptionalObjectAndNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `class D { n: string; tag: number; constructor(n: string, tag: number) { this.n = n; this.tag = tag; } getName(): string { return this.n; } }
function defs(on: boolean): D[] | undefined { return on ? [new D("a", 1), new D("b", 2)] : undefined; }
class Probe {
 static calls = 0;
 static count(xs: D[]): D[] { Probe.calls++; return xs; }
 static run(): string {
  const out: string[] = [];
  for (const [on, k] of [[true, "b"], [true, "z"], [false, "a"]] as [boolean, string][]) {
   const all = defs(on);
   const found = all && Probe.count(all).find((x) => x.getName() === k);
   out.push(found === undefined ? "none" : found.getName() + found.tag);
   const m = all && new Map(all.map((x): [string, number] => [x.getName(), x.tag]));
   out.push(m === undefined ? "nomap" : String(m.size));
  }
  return out.join(",") + "|" + Probe.calls;
 }
}`})
}

// frontend-provenance.mjs enumerateObjects: `Class.prototype.m.call(recv)`
// runs the class's own method (here a generator) with this = recv.
func TestGogenA22PrototypeCallNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `interface IReg { getObjects(): Generator<string, void, undefined>; label(prefix: string, n: number): string }
class Reg implements IReg {
 items: string[] = [];
 tag = "r";
 *getObjects(): Generator<string, void, undefined> { for (const x of this.items) yield this.tag + x; }
 label(prefix: string, n: number): string { return prefix + this.tag + n; }
}
class Sub extends Reg { constructor() { super(); this.tag = "s"; } }
class Probe {
 static viaInterface(reg: IReg): string { return [...Reg.prototype.getObjects.call(reg)].join(";"); }
 static run(): string {
  const r = new Reg(); r.items.push("a", "b");
  const s = new Sub(); s.items.push("c");
  const all = [...Reg.prototype.getObjects.call(r)].filter((x) => x !== "rb");
  return all.join(",") + "|" + [...Reg.prototype.getObjects.call(s)].join(",") + "|" + Reg.prototype.label.call(s, "p", 2) + "|" + Probe.viaInterface(r);
 }
}`})
}

// An override in a subclass makes the virtual call differ from the class's
// own method: refused at the call.
func TestGogenA22PrototypeCallOverrideRefused(t *testing.T) {
	dir := t.TempDir()
	src := `class Reg { tag = "r"; label(): string { return "R" + this.tag; } }
class Sub extends Reg { label(): string { return "S"; } }
export class Probe { static run(): string { return Reg.prototype.label.call(new Sub()); } }
`
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, diags, err := p.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Category == "unsupported-call" && strings.Contains(d.Message, "overrides it") && strings.HasPrefix(d.Loc, "probe.ts:3:") {
			return
		}
	}
	t.Fatalf("override not refused: %v", diags)
}
