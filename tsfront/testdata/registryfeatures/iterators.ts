export interface IObject { getName(): string; }
export class Item implements IObject {
  constructor(private name: string) {}
  getName(): string { return this.name; }
}
export class SnapshotRegistry {
  private objects: {[name: string]: {[type: string]: IObject}} = {};
  private objectsByType: {[type: string]: {[name: string]: IObject}} = {};
  public add(name: string, type: string, object: IObject): void {
    if (this.objects[name] === undefined) { this.objects[name] = {}; }
    if (this.objectsByType[type] === undefined) { this.objectsByType[type] = {}; }
    this.objects[name][type] = object;
    this.objectsByType[type][name] = object;
  }
  public* getObjects(): Generator<IObject, void, undefined> {
    for (const name in this.objects) {
      for (const type in this.objects[name]) { yield this.objects[name][type]; }
    }
  }
  public* getObjectsByType(type: string): Generator<IObject, void, undefined> {
    for (const name in this.objectsByType[type] || []) { yield this.objectsByType[type][name]; }
  }
  public* getFiles(): Generator<IObject, void, undefined> { throw new Error("unreached"); }
}
export class Definitions {
  private all: {[name: string]: IObject} = {};
  public add(name: string, object: IObject): void { this.all[name] = object; }
  public* getAll(): Generator<IObject, void, undefined> {
    for (const name in this.all) { yield this.all[name]; }
  }
}
export class IteratorProbe {
  public static run(): string {
    const r = new SnapshotRegistry();
    const a = new Item("A");
    const b = new Item("B");
    const c = new Item("C");
    r.add("ZA", "CLAS", a);
    r.add("ZB", "INTF", b);
    r.add("ZA", "PROG", c);
    let inventory = "";
    for (const object of r.getObjects()) { inventory += object.getName(); }
    let selected = "";
    for (const object of r.getObjectsByType("INTF")) { selected += object.getName(); }
    let empty = 0;
    for (const object of r.getObjectsByType("XSLT")) { empty++; }
    r.add("ZB", "INTF", c);
    let replaced = "";
    for (const object of r.getObjects()) { replaced += object.getName(); }
    const definitions = new Definitions();
    definitions.add("Z_A", a);
    definitions.add("Z_B", b);
    definitions.add("Z_A", c);
    let methods = "";
    for (const object of definitions.getAll()) { methods += object.getName(); }
    let identity = true;
    for (const object of r.getObjectsByType("CLAS")) { identity = identity && object === a; }
    return `${inventory}/${selected}/${empty}/${replaced}/${methods}/${identity}`;
  }
}
