export class Item {
  constructor(public name: string) {}
}
export class ArrayProbe {
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
    return `${zero}/${large}/${decimal === -2.5}/${missing === undefined}/${nums.length}/${alias.length}|${blank}/${x}/${absent === undefined}|${no}/${yes}/${undef === undefined}|${first === a}/${second === b}/${none === undefined}/${refs.length}`;
  }
}
