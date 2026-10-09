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
    const key: any = flag ? "ab" : 1;
    const word: any = flag ? "abc" : 7;
    const props = `${word.length}/${word.foo === undefined}/${XMLProbe.row0(flag)}/${XMLProbe.fromBag(flag)}`;
    let upper = props + (typeof key === "string" ? key.toUpperCase() : "n");
    const doc: any = XMLProbe.parse(flag ? "<a><ID>S</ID><KEY>k1</KEY><LEN>12x</LEN></a>" : "<a><ID>S</ID><KEY><b>x</b></KEY></a>");
    const row: any = doc?.a;
    if (row?.ID === "S" && typeof row.KEY === "string") { upper += row.KEY.toUpperCase(); } else { upper += "-"; }
    upper += row.LEN === undefined ? "none" : `${parseInt(row.LEN, 10)}`;
    const r = /a.c/i;
    const g = new RegExp("x/y", "gi");
    let hits = "";
    if (r.exec(flag ? "ABC" : "zz")) { hits += "e"; }
    if (!r.exec("zz")) { hits += "n"; }
    return `${upper}/${r.toString()}/${g.toString()}/${hits}|${Array.isArray(list)}/${Array.isArray(x)}/${Array.isArray(missing)}|${typeof x}/${x === null}/${x === 0}/${!!x}/${x ?? "fallback"}|${typeof bool}/${!!bool}/${bool}|${missing === undefined}/${missing === null}/${missing ?? "fallback"}|${!!empty}`;
  }
  private static constValue(v: string | {[index: string]: string} | undefined): string | {[index: string]: string} | undefined { return v; }
  private static fromBag(flag: boolean): string {
    const bag: any = {};
    bag["A"] = "x";
    const val = XMLProbe.constValue(flag ? bag : "plain");
    if (typeof val === "string") { return val; }
    if (typeof val === "object" && val["A"] !== undefined) { return val["A"]; }
    return "none";
  }
  private static row0(flag: boolean): string {
    const doc: any = XMLProbe.parse(flag ? "<a><B>text</B></a>" : "<a><B><C>x</C></B></a>");
    return doc?.a?.B?.C ?? "none";
  }
  public static parse(xml: string): unknown { throw new Error("external XML adapter"); }
}
