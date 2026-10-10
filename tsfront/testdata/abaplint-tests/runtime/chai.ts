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
  public equal(expected: any, message = ""): void {
    this.check(this.value === expected, "expected " + String(this.value) +
      (this.negated ? " not to equal " : " to equal ") + String(expected), message);
  }
  public equals(expected: any, message = ""): void { this.equal(expected, message); }
  public instanceof(expected: new () => any, message = ""): void {
    this.check(this.value instanceof expected, "instanceof check", message);
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
