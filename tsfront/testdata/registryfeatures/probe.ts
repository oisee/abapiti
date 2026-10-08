enum Phase { First = "same", Second = "same", Last = "last" }
enum Visibility { Private = 1, Protected, Public, Alias = 1 }
interface AsyncProbe { parseAsync(): Promise<Probe>; }
interface BridgeAPI { dead(value: any): string; }
export class ExcludedBridge implements BridgeAPI {
  public dead(value: Probe): string { return "original body"; }
}
export class Probe implements AsyncProbe {
  public static createBridge(): BridgeAPI { return new ExcludedBridge(); }
  private static calls: number = 0;
  private static next(): number { this.calls++; return 1; }
  private static onlyUndefined() { return undefined; }
  public static clockProbe(): number { return Date.now(); }
  public static run(raw: string, needle: string, n: number, flag: boolean): string {
    const probe = new Probe();
    if (!(probe instanceof Probe)) { throw new Error("constructor identity"); }
    const values: {[key: string]: number} = {first: 1, second: 2};
    const alias = values;
    const removed = delete values["first"];
    const absent = delete values["missing"];
    values["third"] = 3;
    const emptyMap: Map<string, number> = new Map();
    const emptySet: Set<string> = new Set();
    const emptyArray: string[] = [];
    const snapshot = Object.values(Phase);
    snapshot.reverse();
    let numericValues = "";
    for (const value of Object.values(Visibility)) {
      numericValues += `${typeof value}:${value};`;
    }
    const phase: string = flag ? "Last" : "missing";
    const optional: number | undefined = flag ? 0 : undefined;
    const maybeArray: string[] | undefined = flag ? [raw] : undefined;
    const fallback = (maybeArray ?? []).length + (maybeArray || []).length;
    const branch: string[] = flag ? [] : [needle];
    const less = optional < 1;
    const greater = optional >= 0;
    const reverse = -1 <= optional;
    const absentNumber: number | undefined = flag ? undefined : undefined;
    const bothAbsent = absentNumber >= absentNumber;
    const standalone = Probe.onlyUndefined() === undefined;
    Probe.calls = 0;
    const eager = absentNumber < Probe.next();
    const equalZero = optional === 0;
    const unequalZero = optional !== 0;
    return ` first ${raw}\n${n}/${flag} last |${raw.includes(needle)}|${removed}/${absent}|${Object.keys(alias).join(",")}|${Object.keys(Phase).join(",")}/${Phase[phase]}/${Object.values(Phase).join(",")}|${emptyMap.size}/${emptySet.size}/${emptyArray.length}|${equalZero}/${unequalZero}|${optional}|${fallback}/${branch.length}/${less}/${greater}/${reverse}/${bothAbsent}/${standalone}/${eager}/${Probe.calls}|${Object.keys(Visibility).join(",")}/${Visibility.Private}/${Visibility[Visibility.Public]}/${numericValues}`;
  }
  public async parseAsync(): Promise<Probe> { return this; }
  public static dead(): number { return new Date().getTime(); }
}
