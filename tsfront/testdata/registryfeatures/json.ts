export class JSONProbe {
  public static parse(text: string): unknown { throw new Error("external JSON adapter"); }
  public static observe(text: string): string {
    const value: any = JSONProbe.parse(text);
    return `${value.global.files}/${value.syntax.version}/${value.rules.unknown_rule}/${value.list[1]}/${value.n === null}/${value.missing === undefined}`;
  }
}
