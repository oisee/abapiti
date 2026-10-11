export class SwitchProbe {
  static trace: string = "";
  static subject(c: string): string { SwitchProbe.trace += "S"; return c; }
  static grouped(c: string): string {
    let body = "";
    switch (SwitchProbe.subject(c)) {
      case " ": case "\t": body = "whitespace"; break;
      default: body = "fallback"; break;
      case "a": body = "a"; break;
      case "b": case "c": body = "bc"; break;
    }
    return body;
  }
  static run(): string {
    SwitchProbe.trace = "";
    let result = SwitchProbe.grouped(" ") + "/" + SwitchProbe.grouped("\t");
    result += "/" + SwitchProbe.grouped("a") + "/" + SwitchProbe.grouped("b");
    result += "/" + SwitchProbe.grouped("c") + "/" + SwitchProbe.grouped("z");
    return result + "/" + SwitchProbe.trace;
  }
}
