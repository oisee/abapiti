package tsfront

import "testing"

// frontend.mjs resolveStatic: an Error subclass travels in a helpers record
// (shorthand property), is destructured and constructed by the callee, and
// the caller's catch tests instanceof against the module class.
func TestGogenA21ClassValueRecordNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"lib.ts": `export class Unsupported extends Error {}
export const upper = (s: string): string => s.toUpperCase();
export function notInProgram(missing: string[], cls: string, message: string): Unsupported { missing.push(cls); return new Unsupported(message); }`, "probe.ts": `import {Unsupported, upper, notInProgram} from "./lib.ts";
class Other extends Error {}
type Helpers = Pick<typeof import("./lib.ts"), "upper" | "Unsupported" | "notInProgram"> & {extra: number};
function lowerStatic(owner: string, attr: string, deps: Helpers): string {
  const {upper, Unsupported, notInProgram} = deps;
  if (owner === "") throw new Unsupported(attr + ": no owner");
  if (owner === "m") throw notInProgram([], owner, "missing " + attr);
  if (attr === "x") throw new Other("other " + owner);
  return upper(owner) + "=>" + upper(attr);
}
function resolveStatic(owner: string, attr: string): string {
  return lowerStatic(owner, attr, {upper, Unsupported, notInProgram, extra: 1});
}
class Probe {
 static run(): string {
  const out: string[] = [];
  for (const [o, a] of [["zcl", "attr"], ["", "a"], ["q", "x"], ["m", "y"]]) {
   try { out.push(resolveStatic(o, a)); }
   catch (e) { out.push((e instanceof Unsupported ? "U:" : e instanceof Other ? "O:" : "?:") + (e as Error).message); }
  }
  return out.join(",");
 }
}`})
}

// frontend-classes-native.mjs nativeMeSig: a literal lookup table read with a
// key taken from a closed string union.
func TestGogenA21RecordElementAccessNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `interface Type { readonly k: string; readonly len?: number }
const S: Type = {k: "string"}, XS: Type = {k: "xstring"}, I: Type = {k: "i"};
interface NativeMe { fn: string; params: [string, "importing" | "exporting", "string" | "xstring" | "i", boolean?][] }
const NATIVE: Map<string, NativeMe> = new Map([
  ["A", {fn: "fa", params: [["DATA", "importing", "string"], ["N", "importing", "i", true], ["BUFFER", "exporting", "xstring"]]}],
]);
class Probe {
 static run(): string {
  const n = NATIVE.get("A")!;
  const T = {string: S, xstring: XS, i: I};
  const own = n.params.map(([pn, dir, t, optional]) => ({name: pn, dir, type: T[t], optional: optional === true}));
  return own.map((p) => p.name + ":" + p.dir + ":" + p.type.k + ":" + p.optional + ":" + (p.type === S)).join(",");
 }
}`})
}

// Optional trailing tuple elements: omitted ones read as undefined, through
// annotations and through the checker's inferred tuple types.
func TestGogenA21OptionalTupleNodeOracle(t *testing.T) {
	a19Check(t, map[string]string{"probe.ts": `type Row = [string, "string" | "i", boolean?];
function pick(flag: boolean): [string, number?] { if (flag) return ["on", 2]; return ["off"]; }
class Probe {
 static run(): string {
  const params: Row[] = [["a", "i", true], ["b", "string"], ["c", "i", false]];
  const T = {string: "S", i: "I"};
  const out = params.map(([pn, t, o]) => pn + T[t] + (o === true) + (o === undefined));
  for (const f of [true, false]) {
   const [label, n] = pick(f);
   const r = pick(f);
   out.push(label + String(n) + String(r[1] === undefined) + r[0]);
  }
  return out.join(",");
 }
}`})
}
