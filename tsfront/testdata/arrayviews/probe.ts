export class Probe {
  public static pushOnRoot(): string {
    const s = ["A", "B", "C"];
    const t = s.splice(1);
    s.push("X");
    return t[0] + "|" + t.length + "|" + s.length + "|" + s[0] + s[1];
  }
  public static loops(): string {
    const s = ["A", "B", "C"];
    const t = s.splice(1);
    let r = "";
    for (const x of t) {
      r = r + x;
    }
    for (const y of s) {
      r = r + y;
    }
    return r;
  }
  public static write(): string {
    const s = ["A", "B", "C"];
    const t = s.splice(1);
    t[1] = "X";
    return t[1] + t[0] + s[0] + s.length;
  }
  public static chain(): string {
    const s = ["A", "B", "C", "D"];
    const t = s.splice(1);
    const u = t.splice(1);
    u.push("E");
    return s.length + "|" + t.length + "|" + u.length + "|" + t[0] + u[0] + u[1] + u[2];
  }
  public static single(): string {
    const s = ["A"];
    const t = s.splice(1);
    return s.length + "|" + t.length + "|" + s[0];
  }
}
