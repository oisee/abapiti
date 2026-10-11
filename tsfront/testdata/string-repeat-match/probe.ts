export class Probe {
  public static run(): string {
    const groups: RegExpMatchArray | null = "zabz".match(/(a)(b)/);
    const missing = "xyz".match(/(a)(b)/);
    const global = "aaba".match(/a/g);
    const amps = ("&&&".match(/&/g) || []).length;
    const proxy = "<CLSPROXY>X</CLSPROXY>".match("<CLSPROXY>(.)</CLSPROXY>");
    return `${"xy".repeat(0)}/${"xy".repeat(1)}/${"xy".repeat(3)}/${"".repeat(4)}/${"x".repeat(2.9)}/${"x".repeat(-0.9)}:${groups?.[0]}/${groups?.[1]}/${groups?.[2]}/${groups?.index}/${groups?.length}:${missing === null}:${global?.join(",")}/${global?.length}:${amps}:${proxy?.[1]}`;
  }
}
