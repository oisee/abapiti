export class Probe {
  public static run(raw: string, needle: string, n: number, flag: boolean): string {
    const probe = new Probe();
    if (!(probe instanceof Probe)) { throw new Error("constructor identity"); }
    const values: {[key: string]: number} = {first: 1, second: 2};
    const alias = values;
    const removed = delete values["first"];
    const absent = delete values["missing"];
    values["third"] = 3;
    return ` first ${raw}\n${n}/${flag} last |${raw.includes(needle)}|${removed}/${absent}|${Object.keys(alias).join(",")}`;
  }
  public static dead(): number { return new Date().getTime(); }
}
