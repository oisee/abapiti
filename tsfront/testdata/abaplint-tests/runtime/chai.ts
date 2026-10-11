// Fluent assertion chains are fields, rather than Proxy-based magic.
export class Assertion {
  public to: Assertion;
  public be: Assertion;
  public have: Assertion;
  public not: Assertion;
  public constructor(private value: any, private message = "", private negated = false,
                     pair = true) {
    this.to = this;
    this.be = this;
    this.have = this;
    this.not = this;
    if (pair) {
      this.not = new Assertion(value, message, !negated, false);
      this.not.not = this;
    }
  }
  private check(ok: boolean, detail: string, message = ""): void {
    if (ok === this.negated) {
      throw new Error((message || this.message || "assertion failed") + ": " + detail);
    }
  }
  private display(value: any): string {
    if (typeof value === "object") { return "[object]"; }
    if (typeof value === "function") { return "[function]"; }
    return String(value);
  }
  public equal(expected: any, message = ""): void {
    const ok = this.value === expected;
    // Do not perform ToPrimitive merely to format a successful assertion.
    if (ok === this.negated) {
      this.check(ok, "expected " + this.display(this.value) +
        (this.negated ? " not to equal " : " to equal ") + this.display(expected), message);
    }
  }
  public equals(expected: any, message = ""): void { this.equal(expected, message); }
  public greaterThan(expected: number, message = ""): void {
    if (typeof this.value !== "number") { throw new Error("greaterThan requires a number"); }
    this.check((this.value as number) > expected, "greaterThan check", message);
  }
  public instanceof(expected: new () => any, message = ""): void {
    const object = typeof this.value === "object" && this.value !== null && this.value !== undefined;
    this.check(object && (this.value as object) instanceof expected, "instanceof check", message);
  }
  public contain(expected: any, message = ""): void {
    if (typeof this.value === "string") {
      this.check(this.value.includes(String(expected)), "contain check", message);
    } else {
      this.check((this.value as any[]).includes(expected), "contain check", message);
    }
  }
  public include(expected: any, message = ""): void { this.contain(expected, message); }
  public length(expected: number, message = ""): void {
    if (typeof this.value === "string") {
      this.check(this.value.length === expected, "length check", message);
    } else {
      this.check((this.value as any[]).length === expected, "length check", message);
    }
  }
  // Reading this property asserts; never check it eagerly in the constructor.
  public get undefined(): boolean {
    this.check(this.value === undefined, "undefined check");
    return true;
  }
}
export function expect(value: any, message = ""): Assertion { return new Assertion(value, message); }
export namespace expect {
  export function fail(message: string): never { throw new Error(message); }
}
