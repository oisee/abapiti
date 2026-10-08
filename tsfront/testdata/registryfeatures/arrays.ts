export class Item {
  constructor(public name: string) {}
}
export class ArrayProbe {
  private static visits = 0;
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
