export class RefusedSwitch {
  static run(c: string): string {
    let body = "";
    switch (c) {
      case " ": case "\t": body += "whitespace";
      case "a": body += "a"; break;
      default: body = "fallback"; break;
    }
    return body;
  }
}
