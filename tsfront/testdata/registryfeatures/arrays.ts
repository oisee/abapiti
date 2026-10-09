export interface Named { getName(): string; left(): boolean; }
export interface OtherNamed { getName(): string; right(): boolean; }
export interface NumberRecord { [name: string]: number; }
export class IdentityBase {
  constructor(private token: string) {}
  public getToken(): string { return this.token; }
}
export interface ChildView extends IdentityBase { getOwn(): string; }
export interface GrandView extends ChildView { getGrand(): string; }
export class GrandItem extends IdentityBase implements GrandView {
  constructor(token: string) { super(token); }
  public getOwn(): string { return "own"; }
  public getGrand(): string { return "grand"; }
}
export class SecondGrandItem extends IdentityBase implements ChildView {
  public getOwn(): string { return "second"; }
}
export class OtherItem implements OtherNamed {
  constructor(public name: string) {}
  public getName(): string { return this.name; }
  public right(): boolean { return true; }
}
export class Item implements Named {
  constructor(public name: string) {}
  public getName(): string { return this.name; }
  public left(): boolean { return true; }
}
export class ArrayProbe {
  private static matchReceiver(): string { ArrayProbe.matchOrder += "r"; return "ZTEST"; }
  private static matchPattern(): RegExp { ArrayProbe.matchOrder += "p"; return /^Z/; }
  private static matchOrder = "";
  public static matchTests(): string {
    const name = "ZTEST";
    const missing = name.match(/^Y/) === null;
    const present = name.match(/^Z/) !== null;
    let truth = "";
    if (name.match(/test$/i)) { truth = "yes"; }
    const effects = ArrayProbe.matchReceiver().match(ArrayProbe.matchPattern()) !== null;
    return `${missing}/${present}/${truth}/${effects}/${ArrayProbe.matchOrder}`;
  }
  private static repeatIndent(indent: number): string { return "&nbsp".repeat(indent * 2); }
  public static indentation(): string {
    return `${ArrayProbe.repeatIndent(0)}|${ArrayProbe.repeatIndent(2)}|${Number.MAX_SAFE_INTEGER}|${Number.MIN_SAFE_INTEGER}`;
  }
  private static optionalIdentity(base?: IdentityBase): string {
    if (base === undefined) { return "absent"; }
    return base.getToken();
  }
  private static unionIdentity(value: GrandItem | SecondGrandItem): string {
    return ArrayProbe.optionalIdentity(value);
  }
  private static optionalViewIdentity(value?: ChildView): string { return ArrayProbe.optionalIdentity(value); }
  private static narrowedBranded(base?: IdentityBase): string {
    if (!(base instanceof GrandItem) && !(base instanceof SecondGrandItem)) { return "absent"; }
    return ArrayProbe.unionIdentity(base);
  }
  public static brandedOptional(): string {
    const first = new GrandItem("first");
    const second = new SecondGrandItem("second");
    const iface: ChildView = first;
    return `${ArrayProbe.optionalIdentity(iface)}/${ArrayProbe.optionalViewIdentity()}/${ArrayProbe.optionalViewIdentity(iface)}/${ArrayProbe.unionIdentity(first)}/${ArrayProbe.unionIdentity(second)}/${ArrayProbe.narrowedBranded(first)}/${ArrayProbe.narrowedBranded(second)}/${ArrayProbe.narrowedBranded()}/${ArrayProbe.narrowedBranded(new IdentityBase("plain"))}`;
  }
  public static codeUnits(): string {
    const text = "A~\u00fe\u00ff\u0100\u0101\u20ac\ud83d\ude00\t";
    let out = "";
    for (let index = 0; index < text.length; index++) {
      out += `${text.charCodeAt(index)},`;
    }
    return out;
  }
  public static splice(): string {
    const starts = [0, 1, 2, 4, 5, 9, -1, -2, -9];
    let out = "";
    for (const start of starts) {
      const items = ["a", "b", "c", "d", "e"];
      const alias = items;
      const rest = items.splice(start);
      items.push("x");
      rest.push("y");
      out += `${alias.join("")}:${rest.join("")};`;
    }
    const first = new Item("first");
    const refs = [first, new Item("second"), new Item("third")];
    const tail = refs.splice(1);
    tail.push(new Item("fourth"));
    return `${out}${refs.length}/${tail.length}/${refs[0] === first}/${tail[0].name}`;
  }
  public static sliceAndUnshift(): string {
    const text = "abcde";
    const empty: number[] = [];
    const size = empty.unshift(0);
    const alias = empty;
    const size2 = alias.unshift(3);
    const item = new Item("same");
    const refs: Item[] = [];
    refs.unshift(item);
    return `${text.slice()}/${text.slice(-2)}/${text.slice(-20, 3)}/${text.slice(4, 2)}/${text.slice(2, -1)}/${text.slice(99)}/${text.slice(1.9, 3.9)}/${size}/${size2}/${empty[0]}/${empty[1]}/${refs[0] === item}`;
  }
  public static continueUpdates(): string {
    let visits = 0;
    let updates = 0;
    let index = 0;
    for (; index < 4; index = ArrayProbe.nextIndex(index, updates++)) {
      if (index === 1) { continue; }
      let inner = 0;
      while (inner < 2) { inner++; if (inner === 1) { continue; } visits++; }
      for (const value of [0, 1]) { if (value === 0) { continue; } visits++; }
      for (let nested = 0; nested < 2; nested++) { if (nested === 0) { continue; } visits++; }
      visits++;
    }
    return `${index}/${updates}/${visits}`;
  }
  private static nextIndex(index: number, updates: number): number { return index + 1; }
  private static postCounter = 11;
  private static receiverCalls = 0;
  private static indexCalls = 0;
  private static absent: string | undefined = undefined;
  private static nullValue: string | null = null;
  private static numbers(values: number[]): number[] { ArrayProbe.receiverCalls++; return values; }
  private static postIndex(): number { ArrayProbe.indexCalls++; return 0; }
  public static postfix(): string {
    let local = 3;
    const before = local++;
    const previous = ArrayProbe.postCounter--;
    const values: number[] = [7];
    const indexed = ArrayProbe.numbers(values)[ArrayProbe.postIndex()]++;
    const pair = local++ + local++;
    return `${before}/${local}/${previous}/${ArrayProbe.postCounter}/${indexed}/${values[0]}/${ArrayProbe.receiverCalls}/${ArrayProbe.indexCalls}/${pair}/${ArrayProbe.absent === undefined}/${ArrayProbe.nullValue === null}`;
  }

  private static optionalItems(): Item[] | undefined { return undefined; }
  public static denseCallbacks(): string {
    const texts: string[] = ["", "b", "a"];
    const found = texts.find(text => text === "");
    const missing = texts.find(text => text === "missing");
    const every = texts.every(text => text.length > 0);
    const empty: string[] = [];
    const emptyEvery = empty.every(text => text.length > 0);
    const numbers: number[] = [0, 7];
    let sum = 0;
    const result = numbers.forEach((value, index, array) => {
      sum += value;
      array[index] = value + 1;
    });
    const refs: Item[] = [new Item("a"), new Item("b")];
    let names = "";
    refs.forEach((item, index, array) => {
      if (index === 0) { array[index] = new Item("x"); }
      names += item.name;
    });
    let visits = 0;
    ArrayProbe.optionalItems()?.forEach(item => visits++);
    return `${found === ""}/${missing === undefined}/${every}/${emptyEvery}/${sum}/${numbers[0]}/${numbers[1]}/${result === undefined}/${names}/${refs[0].name}/${visits}`;
  }
  private static baseToken(value: IdentityBase): string { return value.getToken(); }
  private static childToken(value: ChildView): string { return value.getToken(); }
  public static interfaceHeritage(): string {
    const item = new GrandItem("token");
    const grand: GrandView = item;
    const child: ChildView = grand;
    return `${grand.getToken()}/${grand.getOwn()}/${child.getOwn()}/${ArrayProbe.baseToken(grand)}/${ArrayProbe.childToken(grand)}/${grand === item}`;
  }
  public static namedRecord(): string {
    const values: NumberRecord = {first: 0, second: 7};
    const alias = values;
    alias["third"] = 11;
    const missing = values["missing"];
    return `${values["first"]}/${values["third"]}/${missing === undefined}/${Object.keys(values).join(",")}`;
  }
  private static map = new Map<string, number>();
  private static values = new Set<string>();
  public static staticCollections(): string {
    const before = ArrayProbe.map.size;
    ArrayProbe.map.set("key", 11);
    ArrayProbe.values.add("value");
    ArrayProbe.values.add("value");
    return `${before}/${ArrayProbe.map.size}/${ArrayProbe.map.get("key")}/${ArrayProbe.values.size}`;
  }
  private static visits = 0;
  private static readUnion(value: Named | OtherNamed): string { return value.getName(); }
  private static isItem(value: Named | OtherNamed): value is Named { return value instanceof Item; }
  public static unionViews(): string {
    const left: Named = new Item("left");
    const right: OtherNamed = new OtherItem("right");
    const alias: Named | OtherNamed = left;
    return `${ArrayProbe.readUnion(left)}/${ArrayProbe.readUnion(right)}/${alias === left}/${ArrayProbe.isItem(left)}/${ArrayProbe.isItem(right)}`;
  }
  private static appendDuringFind(item: Item, index: number, array: Item[]): boolean {
    ArrayProbe.visits++;
    if (index === 0) { array.push(new Item("appended")); }
    return false;
  }
  private static removeDuringFind(item: Item, index: number, array: Item[]): boolean {
    ArrayProbe.visits++;
    if (index === 0) { array.pop(); array.pop(); }
    return item === undefined;
  }
  public static find(): string {
    const a = new Item("a");
    const b = new Item("b");
    const refs: Item[] = [a, b];
    const first = refs.find(entry => entry === b);
    const none = refs.find(entry => entry.name === "missing");
    ArrayProbe.visits = 0;
    refs.find((entry, index, array) => ArrayProbe.appendDuringFind(entry, index, array));
    const appendedVisits = ArrayProbe.visits;
    const removed: Item[] = [a, b];
    ArrayProbe.visits = 0;
    const missing = removed.find((entry, index, array) => ArrayProbe.removeDuringFind(entry, index, array));
    const removedVisits = ArrayProbe.visits;
    const empty: Item[] = [];
    const emptyResult = empty.find(entry => true);
    return `${first === b}/${none === undefined}/${appendedVisits}/${refs.length}/${missing === undefined}/${removedVisits}/${emptyResult === undefined}`;
  }
  private static index(): number { ArrayProbe.visits++; return 0; }
  public static optionalIndex(): string {
    ArrayProbe.visits = 0;
    const missing: number[] | undefined = undefined;
    const absent = missing?.[ArrayProbe.index()];
    const present: number[] | undefined = [7];
    const value = present?.[ArrayProbe.index()];
    const map: {[key: string]: number} | undefined = {key: 11};
    const entry = map!["key"];
    const missingEntry = map!["missing"];
    return `${absent === undefined}/${value}/${entry}/${missingEntry === undefined}/${ArrayProbe.visits}/${present!.length}`;
  }
  public static shift(): string {
    const nums: number[] = [0, 9007199254740991, -2.5];
    const alias = nums;
    const zero = nums.shift();
    const large = alias.shift();
    const decimal = nums.shift();
    const missing = nums.shift();
    const texts: string[] = ["", "x"];
    const blank = texts.shift();
    const x = texts.shift();
    const absent = texts.shift();
    const flags: boolean[] = [false, true];
    const no = flags.shift();
    const yes = flags.shift();
    const undef = flags.shift();
    const a = new Item("a");
    const b = new Item("b");
    const refs: Item[] = [a, b];
    const first = refs.shift();
    const second = refs.shift();
    const none = refs.shift();
    const maybe: (string | undefined)[] = ["m", undefined, "z", "q"];
    const m = maybe.shift();
    const hole = maybe.shift();
    const q = maybe.pop();
    const at = maybe[0];
    const past = maybe[5];
    const opts = `${m}/${hole === undefined}/${q}/${at}/${past === undefined}/${maybe.length}`;
    return `${opts}|${zero}/${large}/${decimal === -2.5}/${missing === undefined}/${nums.length}/${alias.length}|${blank}/${x}/${absent === undefined}|${no}/${yes}/${undef === undefined}|${first === a}/${second === b}/${none === undefined}/${refs.length}`;
  }
}
