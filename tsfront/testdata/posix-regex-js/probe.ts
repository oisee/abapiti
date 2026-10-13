export class Probe {
  static dynamic(pattern: string, input: string): boolean {
    return new RegExp(pattern).test(input);
  }
  static dynamicI(pattern: string, input: string): boolean {
    return new RegExp(pattern, "i").test(input);
  }
  static dynamicS(pattern: string, input: string): boolean {
    return new RegExp(pattern, "s").test(input);
  }
  static run(): string {
    const dot = /a.b/;
    const space = /^\s*$/;
    const word = /\bx\b/;
    const groups = "a\u00a0\tb".match(/\s+/g);
    const bounded = "éxé".match(/\b(x)\b/);
    return `${dot.test("a\nb")}/${dot.test("a\rb")}/${dot.test("a\u2028b")}/${dot.test("a\u2029b")}/${dot.test("axb")}`
      + `/${"a\nb".match(/a.b/) !== null}/${"axb".match(/a.b/) !== null}`
      + `/${space.test(" ")}/${space.test("\u3000")}/${space.test("\ufeff")}/${space.test(" \r")}/${space.test("\u00a0")}/${space.test("x")}`
      + `/${/\S/.test("\u00a0")}/${/\S/.test("x")}/${word.test("éxé")}/${word.test("a x")}/${/\Bx\B/.test("axa")}/${/\Bx\B/.test("éxé")}`
      + `/${new RegExp(".").test("\n")}/${/a.b/s.test("a\nb")}/${/A.B/i.test("axb")}/${/A.B/i.test("a\rb")}/${/A.B/si.test("a\nb")}`
      + `/${/\bX\b/i.test("éxé")}/${/\bk\b/i.test("K")}/${/\bs\b/i.test("ſ")}`
      + `/${/^[\s]+$/.test("\u3000\ufeff\r")}/${/^[.]+$/.test(".")}/${/a\.b/.test("a.b")}`
      + `/${Probe.dynamic(".", "\n")}/${Probe.dynamic("a.b", "a\rb")}/${Probe.dynamicS("a.b", "a\nb")}`
      + `/${Probe.dynamicI("^\\s*$", "\u3000\ufeff\r")}/${Probe.dynamic("\\S", "\u00a0")}/${Probe.dynamic("^[\\s]+$", "\u00a0\r")}`
      + `/${"a\u00a0\tb".replace(/\s+/g, "_")}/${groups?.join("|")}`
      + `/${bounded?.[0]}/${bounded?.[1]}/${bounded?.index}/${bounded?.length}`;
  }
}
