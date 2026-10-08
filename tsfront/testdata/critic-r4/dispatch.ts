interface Item { value(): string; }
interface Getter { get(): Item; }
interface VoidGetter { take(): void; }
export class Leaf implements Item {
  constructor(public text: string) {}
  value(): string { return this.text; }
}
export abstract class Base<T extends Item> { abstract get(): T; }
export class Node extends Base<Leaf> implements Getter, VoidGetter {
  get(): Leaf { return new Leaf("node"); }
  take(): Leaf { return new Leaf("node"); }
}
export class Sub extends Node {
  get(): Leaf { return new Leaf("sub"); }
  take(): Leaf { return new Leaf("sub"); }
  parent(): string { return super.get().value(); }
}
export class Probe {
  static run(): string {
    const sub = new Sub();
    const node: Node = sub;
    const base: Base<Leaf> = sub;
    const getter: Getter = sub;
    return node.get().value() + "|" + base.get().value() + "|" + getter.get().value() + "|" + sub.parent() + "|" + node.take().value();
  }
}
