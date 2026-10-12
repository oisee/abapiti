// Fluent assertion chains are fields, rather than Proxy-based magic.
export class Assertion {
  public to: Assertion;
  public be: Assertion;
  public have: Assertion;
  public constructor(private value: any, private message = "", private negated = false,
                     private deeply = false) {
    this.to = this;
    this.be = this;
    this.have = this;
  }
  public get not(): Assertion { this.negated = true; return new Assertion(this.value, this.message, true, this.deeply); }
  public get deep(): Assertion { this.deeply = true; return new Assertion(this.value, this.message, this.negated, true); }
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
    const ok = this.deeply ? chaiDeepEqual(this.value, expected) : this.value === expected;
    // Do not perform ToPrimitive merely to format a successful assertion.
    if (ok === this.negated) {
      this.check(ok, "expected " + this.display(this.value) +
        (this.negated ? " not to equal " : " to equal ") + this.display(expected), message);
    }
  }
  public equals(expected: any, message = ""): void { this.equal(expected, message); }
  public eql(expected: any, message = ""): void { this.check(chaiDeepEqual(this.value, expected), "deep equal check", message); }
  public eqls(expected: any, message = ""): void { this.eql(expected, message); }
  public greaterThan(expected: number, message = ""): void {
    if (typeof this.value !== "number") { throw new Error("greaterThan requires a number"); }
    this.check((this.value as number) > expected, "greaterThan check", message);
  }
  public instanceof(expected: new () => any, message = ""): void {
    const object = typeof this.value === "object" && this.value !== null && this.value !== undefined;
    this.check(object && (this.value as object) instanceof expected, "instanceof check", message);
  }
  public instanceOf(expected: new () => any, message = ""): void { this.instanceof(expected, message); }
  // Chai ignores the return value of its synchronous action. The intrinsic
  // invokes only certified parameterless callable storage, without a void cast.
  public throw(expected = "", message = ""): void {
    this.check(chaiThrowMatch(this.value, expected), "throw check", message);
  }
  public contain(expected: any, message = ""): void {
    if (this.deeply) {
      const included = chaiDeepInclude(this.value, expected);
      // Chai's object subset assertion does not assert for an empty subset,
      // even when negated. 2 denotes that no-assertion case.
      if (included !== 2) { this.check(included === 1, "deep include check", message); }
      return;
    }
    if (typeof this.value === "string") {
      this.check(this.value.includes(String(expected)), "contain check", message);
    } else {
      this.check(chaiArrayInclude(this.value, expected), "contain check", message);
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
// Harness intrinsics: only these resolved symbols in tests-only lowering are
// replaced. Unsupported JS representations raise a typed refusal, not equality.
export function chaiDeepEqual(actual: any, expected: any): boolean { throw new Error("chai deep intrinsic unavailable"); }
export function chaiDeepInclude(actual: any, expected: any): number { throw new Error("chai deep intrinsic unavailable"); }
export function chaiThrowMatch(action: any, expected: string): boolean { throw new Error("chai throw intrinsic unavailable"); }
export function chaiArrayInclude(actual: any, expected: any): boolean { throw new Error("chai array include intrinsic unavailable"); }
export function chaiFail(message: string): void { throw new Error(message); }
export function expect(value: any, message = ""): Assertion { return new Assertion(value, message); }
export namespace expect {
  export function fail(message: string): void { chaiFail(message); }
}
