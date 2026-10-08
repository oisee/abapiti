export class XMLProbe {
  public constructor(private xml?: string) {}
  protected getXML(): string | undefined { return this.xml; }
  public raw(): any | undefined { return undefined; }
  public description(): string {
    const parsed = this.raw();
    const vseo = parsed?.abapGit?.["asx:abap"]?.["asx:values"]?.VSEOCLASS;
    return vseo?.DESCRIPT ? vseo.DESCRIPT : "";
  }
  public static tagged(flag: boolean): string {
    const x: any = flag ? 0 : null;
    const bool: any = flag ? false : true;
    const missing: any = undefined;
    const empty: any = "";
    const list: any = flag ? ["a"] : "s";
    return `${Array.isArray(list)}/${Array.isArray(x)}/${Array.isArray(missing)}|${typeof x}/${x === null}/${x === 0}/${!!x}/${x ?? "fallback"}|${typeof bool}/${!!bool}/${bool}|${missing === undefined}/${missing === null}/${missing ?? "fallback"}|${!!empty}`;
  }
  public static parse(xml: string): unknown { throw new Error("external XML adapter"); }
}
