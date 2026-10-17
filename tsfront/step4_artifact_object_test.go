package tsfront

import "testing"

// frontend-session-types.ts ArtifactObject: an interface of methods plus
// optional unknown data slots, implemented by the core's IObject classes.
// frontend.mjs filters registry objects through it and calls getName();
// frontend-inventory.mjs reads the slots by name, including undeclared ones.
// A slot reads the object's own field or prototype member, else undefined.
func TestStep4ArtifactObjectNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IFile { getRaw(): string }
export class MemoryFile implements IFile { private readonly raw: string; constructor(raw: string) { this.raw = raw; } getRaw(): string { return this.raw; } }
export interface IObject { getType(): string; getName(): string; isDirty(): boolean; getFiles(): IFile[]; setDirty(): void }
export abstract class AbstractObject implements IObject {
  private readonly name: string; protected dirty = true; private readonly files: IFile[] = [];
  constructor(name: string) { this.name = name; this.files.push(new MemoryFile(name)); }
  abstract getType(): string;
  getName(): string { return this.name; }
  isDirty(): boolean { return this.dirty; }
  setDirty(): void { this.dirty = true; }
  getFiles(): IFile[] { return this.files; }
}
export class Group extends AbstractObject { private modules: string[] | undefined = undefined; getType(): string { return "FUGR"; } load(): void { this.modules = ["A", "B"]; } getModules(): string[] | undefined { return this.modules; } }
export class Clas extends AbstractObject { public syntaxResult: string | undefined; private def: Group | undefined = undefined; getType(): string { return "CLAS"; } run(g: Group): void { this.syntaxResult = "ok"; this.def = g; this.dirty = false; } }
export class Registry { readonly objs: IObject[] = [new Group("g1"), new Clas("c1"), new Group("g2")]; *getObjects(): Generator<IObject> { yield* this.objs; } }
export interface ArtifactObject { getType(): string; getName(): string; getFiles(): readonly IFile[]; isDirty?(): boolean; syntaxResult?: unknown; def?: unknown; definition?: unknown }
`, `import {Group, Clas, Registry} from "./types.ts";
const artifactFields = ["syntaxResult", "def", "definition", "modules", "getName", "dirty"];
/** @param {import("./types.ts").ArtifactObject} obj @returns {unknown[]} */ const identity = (obj) => artifactFields.map((name) => obj[name]);
/** @param {unknown[] | undefined} a @param {unknown[]} b @returns {boolean | undefined} */ const same = (a, b) => a?.every((value, i) => value === b[i]);
const /** @type {WeakMap<Pick<import("./types.ts").ArtifactObject, "getType" | "getName">, string>} */ keys = new WeakMap();
/** @param {Pick<import("./types.ts").ArtifactObject, "getType" | "getName">} obj @returns {string} */ function objectKey(obj) {
  if (!keys.has(obj)) keys.set(obj, obj.getType() + " " + obj.getName().toUpperCase());
  return keys.get(obj);
}
/** @param {Registry} reg @param {(obj: import("./types.ts").ArtifactObject) => boolean} predicate @returns {import("./types.ts").ArtifactObject[]} */
function enumerateObjects(reg, predicate = () => true) { return [...reg.getObjects()].filter(predicate); }
/** @param {import("./types.ts").ArtifactObject & {tag?: string}} o @returns {string} */ const tagged = (o) => o.getName() + (o.tag ?? "-");
/** @param {import("./types.ts").IObject[]} objects @returns {string} */ const inventory = (objects) => objects.map((o) => o.getName() + (o.isDirty() ? "*" : "")).join("+");
/** @template {import("./types.ts").IObject} T @param {Iterable<T>} objects @param {Map<string, string>} seen @returns {Generator<T>} */ function* dirtyOnes(objects, seen) {
  for (const o of objects) { seen.set(o.getName(), o.getType()); if (o.isDirty()) yield o; }
}
/** @param {import("./types.ts").ArtifactObject} obj @returns {boolean} */
const built = (obj) => Boolean(obj.syntaxResult || obj.def || obj.definition) || obj.isDirty?.() === false;
export class Probe {
  /** @returns {string} */ static run() {
    const reg = new Registry();
    const wanted = ["g2"];
    const groups = enumerateObjects(reg, (o) => o instanceof Group && wanted.includes(o.getName().toLowerCase()));
    const names = enumerateObjects(reg).map((o) => o.getType() + ":" + o.getName() + ":" + o.getFiles()[0].getRaw() + ":" + built(o) + ":" + objectKey(o));
    for (const g of enumerateObjects(reg, (o) => o instanceof Group)) names.push(g.getName().toLowerCase() + String(g.getModules()?.length));
    for (/** @type {Clas} */ const c of enumerateObjects(reg, (o) => o instanceof Clas && o.getName() === "c1")) names.push(c.getName() + c.getType());
    names.push(inventory(enumerateObjects(reg)));
    /** @type {Map<string, string>} */ const seen = new Map();
    for (const obj of dirtyOnes(enumerateObjects(reg), seen)) names.push(obj.getName() + obj.getType());
    names.push(String(seen.size));
    for (const obj of enumerateObjects(reg, (o) => o instanceof Group || o instanceof Clas)) {
      const isClas = obj instanceof Clas;
      if (!(obj instanceof Group) && !isClas) continue;
      /** @type {Clas | undefined} */ const c = obj instanceof Clas ? obj : undefined;
      names.push(String(c?.syntaxResult));
    }
    for (const c of enumerateObjects(reg, /** @param {import("./types.ts").IObject} o */ (o) => o instanceof Clas && o.getName() === "c1")) names.push(c.getType());
    const before = enumerateObjects(reg).map(identity);
    /** @type {Clas} */ (reg.objs[1]).run(/** @type {Group} */ (reg.objs[0]));
    /** @type {Group} */ (reg.objs[2]).load();
    const after = enumerateObjects(reg).map(identity);
    const flags = before.map((b, i) => String(same(b, after[i])) + "/" + String(same(b, identity(enumerateObjects(reg)[i]))) + "/" + after[i].map((v) => typeof v).join("."));
    return groups.map((g) => g.getName()).join(",") + "|" + names.join(",") + "|" + enumerateObjects(reg).map(built).join(",") + "|" + flags.join(",") + "|" + String(after[1][0]) + String(same(undefined, after[0]));
  }
}
`, false, true)
}
