export class Probe {
  private static pattern(): string { return "(a)(b)"; }
  public static run(): string {
    let out = "";
    const first: RegExpExecArray | null = /(a)(b)/.exec("😀abzab");
    if (first !== null) { out += first.index + ":" + first[0] + ":" + first[1] + ":" + first[2]; }
    const g = new RegExp(Probe.pattern(), "g");
    let match: RegExpExecArray | null = g.exec("zabzab");
    while (match !== null) { out += "|" + match.index + ":" + match[1]; match = g.exec("zabzab"); }
    const again: RegExpMatchArray | null = g.exec("ab");
    if (again !== null) { out += "|reset:" + again.index; }
    const sticky = new RegExp(Probe.pattern(), "y");
    const a = sticky.exec("abab"); const b = sticky.exec("abab"); const c = sticky.exec("abab");
    if (a !== null && b !== null) { out += "|sticky:" + a.index + ":" + b.index + ":" + (c === null); }
    const e = /a*/g;
    const e1 = e.exec("z"); const e2 = e.exec("z");
    if (e1 !== null && e2 !== null) { out += "|empty:" + e1.index + ":" + e2.index; }
    out += "|last:" + g.lastIndex;
    g.lastIndex = 99;
    out += ":" + (g.exec("ab") === null) + ":" + g.lastIndex;
    sticky.lastIndex = 1;
    const at = sticky.exec("zab");
    if (at !== null) { out += "|at:" + at.index + ":" + sticky.lastIndex; }
    const anchored = new RegExp("^a", "g");
    anchored.lastIndex = 1;
    out += "|anchor:" + (anchored.exec("aa") === null);
    const needle = "ab"; const replacement = "X";
    out += "|" + "abab".replace(needle, replacement);
    out += "|" + "abab".replace(new RegExp(Probe.pattern(), "g"), replacement);
    out += "|" + "abab".replace(/(a)(b)/g, replacement);
    const behavior = /\bdefine\s+(?:\w+\s+)*?behavior\s+for\s+([\w/]+)(?:\s+alias\s+(\w+))?/gi;
    const entity = behavior.exec("DEFINE abstract behavior for ZBOOKING");
    if (entity !== null) { out += "|" + entity[1] + ":" + (entity[2] === undefined); }
    const annotated = "SEARCHABLE";
    out += "|cds:" + new RegExp("[.{,]" + annotated + ":TRUE").test("@SEARCH:{SEARCHABLE:TRUE}");
    const stripped = "// define behavior for BAD\nDEFINE behavior for ZONE alias One\nDEFINE abstract behavior for ZTWO".replace(/\/\/.*$/gm, "");
    behavior.lastIndex = 0;
    let item = behavior.exec(stripped);
    while (item !== null) { out += "|entity:" + item[1]; item = behavior.exec(stripped); }
    return out;
  }
}
