// Original-JS oracle fixture for the syntax-closure lowerings. The expected
// strings in oracle.json come from running this file with the pinned upstream
// TypeScript compiler; the ABAP side is tsgo -> HIR -> ABAP of the same file.
class Named {
  constructor(private readonly name: string) {}
  public getName(): string { return this.name; }
}
interface IDef extends Named { kind(): string; }
interface IClassDef extends IDef { isFinal(): boolean; }
class ClassDef extends Named implements IClassDef {
  public kind(): string { return "CLAS"; }
  public isFinal(): boolean { return false; }
}
class IntfDef extends Named implements IDef {
  public kind(): string { return "INTF"; }
}
class Scope {
  private readonly defs: (ClassDef | IntfDef)[] = [];
  public add(def: ClassDef | IntfDef): void { this.defs.push(def); }
  public find(name: string): IClassDef | IDef | undefined {
    return this.defs.find(d => d.getName() === name);
  }
  public all(): IDef[] { return [...this.defs]; }
}
class Counter {
  public row = 0;
  public next(): number { return this.row++; }
}
class Trivial {
  public readonly value: number;
  constructor(value: number) { this.value = value; }
}
type Data = {qualifiedName?: string, derived?: boolean, ddic?: string};
abstract class SiblingBase { public abstract tag(): string; }
class Sibling1 extends SiblingBase { public tag(): string { return "s1"; } }
class Sibling2 extends SiblingBase { public tag(): string { return "s2"; } public only2(): string { return "only2"; } }
type Meta = {[key: string]: number};
interface INodeLike { kindOf(): string; }
abstract class Holder<T extends INodeLike> implements INodeLike {
  protected readonly kids: T[] = [];
  public abstract kindOf(): string;
  public firstKid(): T | undefined { return this.kids[0]; }
}
class TNode implements INodeLike { public kindOf(): string { return "t"; } public concat(): string { return "T"; } }
class ENode extends Holder<ENode | TNode> {
  public constructor(depth: number, flag: boolean) {
    super();
    if (depth > 0) { this.kids.push(flag ? new ENode(depth - 1, flag) : new TNode()); }
  }
  public kindOf(): string { return "e"; }
  public concat(): string { return "E"; }
}
interface IConfigured { getConfig(): void; }
class RuleA implements IConfigured { public getConfig(): {level: number} { return {level: 3}; } }
class RuleB implements IConfigured { public getConfig(): {level: number} { return {level: 5}; } }
class Rel {
  private readonly brand = true;
  constructor(public readonly ordinal: number, public readonly name: string) {}
  public isBranded(): boolean { return this.brand; }
}
type Opts = {release?: Rel, tag?: string};
export class Probe {
  private static readonly defaultRel = new Rel(0, "newest");
  private static isRel(o: Opts | Rel): o is Rel { return o instanceof Rel; }
  public static pick(o: Opts | Rel | undefined): string {
    if (o === undefined || Probe.isRel(o)) {
      const r = o ?? Probe.defaultRel;
      return r.name;
    }
    return o.tag ?? "notag";
  }
  private static readonly cache: Map<string, number> = new Map();
  private static pending: string | undefined = undefined;
  private static readonly seed: Trivial = new Trivial(5);
  private static readonly table: {[name: string]: number} = {"a": 1, "b": 2};
  private counter = 0;
  private readonly deferred: (() => void)[] = [];
  private log: string[] = [];
  private note(text: string): void { this.log.push(text); }
  private static unrelated(x: Named): string {
    if (x instanceof Counter) { return "impossible"; }
    return x.getName();
  }
  private static tagged(v: string | number[]): string {
    if (Array.isArray(v)) { let s = 0; for (const n of v) { s += n; } return `arr:${s}`; }
    return `str:${v}`;
  }
  private static base(x: SiblingBase): SiblingBase { return x; }
  public run(raw: string, needle: string, n: number, flag: boolean): string {
    const scope = new Scope();
    scope.add(new ClassDef("zcl_" + raw));
    scope.add(new IntfDef("zif_" + raw));
    const found = scope.find("zcl_" + raw);
    const missing = scope.find("nope");
    const names = scope.all().map(d => { const upper = d.getName().toUpperCase(); return upper + ":" + d.kind(); });
    const idx = scope.all().findIndex(d => d.kind() === "INTF");
    const allNamed = !scope.all().some(d => !(d.getName().length > 0));
    const flat = [["x", "y"], ["z"]].flatMap(a => a);
    let seen = "";
    let ix = 0;
    for (const d of scope.all()) { seen += `${ix}${d.kind()}`; ix++; }
    const queue = ["q1", "q2", "q3"];
    const first = queue.shift();
    queue.unshift("q0");
    const set = new Set(["b", "a", "b", "c"]);
    set.delete("c");
    const union = [...set, ...Array.from(set.values())];
    const parsed = parseInt(" 42abc", 10) + parseInt("-7", 10) + parseInt("+3", 10);
    const nan = isNaN(parsed);
    const big = Number.MAX_SAFE_INTEGER;
    const sliced = raw.slice(1, -1) + "|" + raw.slice(-2) + "|" + raw.slice(5);
    const ch = raw[0] === undefined ? "none" : raw[0];
    const far = raw[99] === undefined ? "none" : raw[99];
    const replaced = (raw + needle + raw).replace(needle, "_");
    const sorted = ["Z_A", "A/B", "A_B", "A0", "A", "9X", "/X"].sort((a, b) => a.localeCompare(b));
    const counter = new Counter();
    const before = counter.next();
    const after = counter.row;
    let skipped = 0;
    for (let i = 0; i < 6; i++) { if (i % 2 === 0) { continue; } skipped += i; }
    const meta: Meta | undefined = flag ? {k1: 1, k2: 2} : undefined;
    let keys = "";
    for (const k in meta) { keys += k; }
    let text: string | undefined = flag ? raw : undefined;
    let chained = "";
    while (text !== undefined) { chained = text?.toUpperCase(); text = undefined; }
    let fin = "";
    try { fin += "a"; } finally { fin += "b"; }
    let sw = "";
    switch (needle) { case "b": sw = "B"; break; case "x": sw = "X"; break; default: sw = "D"; break; }
    const shadow = raw;
    let inner = "";
    { const shadow = needle; inner = shadow; }
    const data: Data = {qualifiedName: "q", derived: true, ddic: "d"};
    const {derived, ...rest} = data;
    const restName = rest.qualifiedName ?? "none";
    let sib = new Sibling1();
    if (flag) { sib = new Sibling2(); }
    const tagged = Probe.tagged(flag ? [1, 2, 3] : raw);
    let errText = "";
    try { throw new Error(raw); } catch (e) { errText = e.toString(); }
    let emptyErr = "";
    try { throw new Error(""); } catch (e) { emptyErr = e.toString(); }
    const self = this;
    const captured = needle;
    this.deferred.push(() => { self.note(captured + "!"); });
    this.deferred.push(() => { this.counter++; });
    for (const d of this.deferred) { d(); }
    const prim: string = raw;
    const primUndef = prim === undefined;
    const missingPending = Probe.pending === undefined;
    const seed = Probe.seed.value + (Probe.table["b"] ?? 0) + Probe.cache.size;
    const evolving = [];
    evolving.push(...names.slice(0, 1));
    evolving.push("tail");
    const root = new ENode(1, flag);
    const viewed = (root.firstKid()?.concat() ?? "none") + (root.firstKid()?.kindOf() ?? "") + (new ENode(0, flag).firstKid()?.concat() ?? "none");
    const recs: {[k: string]: {[t: string]: boolean}} = {a: {x: true, y: true}, b: {z: true}};
    delete recs["a"]?.["x"];
    delete recs["zz"]?.["q"];
    const recsKeys = Object.keys(recs["a"]).length + ":" + Object.keys(recs).join("");
    if (Object.keys(recs["b"]).length === 1) { delete recs["b"]; } else { delete recs["b"]["z"]; }
    const afterDel = Object.keys(recs).join(",") + Object.keys(recs["a"]).join("");
    const picked = Probe.pick(new Rel(7, raw)) + Probe.pick(undefined) + Probe.pick({tag: needle}) + Probe.pick({release: Probe.defaultRel});
    const box: any = flag ? {DDTEXT: raw, NUM: n} : undefined;
    const boxText: string | undefined = box?.DDTEXT || "";
    const boxMissing: string = box?.MISSING || "dflt";
    const boxNum: number = box?.NUM || -1;
    const rules: IConfigured[] = [new RuleA(), new RuleB()];
    const bag: {[k: string]: any} = {};
    let ruleIndex = 0;
    for (const rule of rules) { bag["r" + ruleIndex] = rule.getConfig(); ruleIndex++; }
    const bagKeys = Object.keys(bag).join("") + (bag["r1"] === undefined ? "u" : "d") + (typeof bag["r1"] === "object" ? "o" : "x");
    const downcast = Probe.base(sib) instanceof Sibling2 ? (Probe.base(sib) as Sibling2).only2() : "not2";
    const typed = `${typeof counter === "boolean"}/${typeof n === "number"}/${typeof flag === "string"}/${typeof sib !== "number"}`;
    const nested: {[k: string]: {[n: string]: string[]}} = {};
    nested["m"] = {};
    nested["m"]["1"] = ["keep", needle, "keep2"];
    nested["m"]["1"] = nested["m"]["1"].filter(v => v !== needle);
    const filtered = nested["m"]["1"].join(",");
    const parts: string[] = [evolving.join("+"), viewed, downcast, typed, filtered,
      found?.getName() ?? "none", found?.kind() ?? "none", missing === undefined ? "absent" : "present",
      names.join(","), `${idx}`, `${allNamed}`, flat.join(""), seen, first ?? "none", queue.join(","), union.join(""),
      `${parsed}`, `${nan}`, `${big}`, sliced, ch, far, replaced, sorted.join(" "), `${before}`, `${after}`, `${skipped}`, keys,
      chained, fin, sw, shadow + inner, restName, `${derived}`, sib.tag(), tagged, errText, emptyErr,
      this.log.join(";"), `${this.counter}`, `${primUndef}`, `${missingPending}`, `${seed}`, Probe.unrelated(new ClassDef("u")), `${n}`, `${flag}`,
      recsKeys, afterDel, picked, boxText, boxMissing, `${boxNum}`, bagKeys,
    ];
    return parts.join("|");
  }
}
