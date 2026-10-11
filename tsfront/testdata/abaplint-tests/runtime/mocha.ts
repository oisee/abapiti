// Test callbacks use the certified settled-promise drain on TS-HG@Go.
// The Node contract runner awaits native promises; real pending work is refused by lowering.
class TestContext {
  public timeout(_milliseconds: number): void {}
}
class TestFailure {
  public constructor(public title: string, public message: string) {}
}
class TestResult {
  public constructor(public title: string, public passed: boolean, public message: string) {}
}
class TestSuite {
  public beforeAll: (() => void)[] = [];
  public afterAll: (() => void)[] = [];
  public beforeEach: (() => void)[] = [];
  public afterEach: (() => void)[] = [];
}
class TestCase {
  public constructor(public title: string, public body: () => void,
                     public only: boolean, public skipped: boolean, public suites: TestSuite[] = []) {}
}
class TestSummary {
  public passed = 0;
  public failed = 0;
  public failures: TestFailure[] = [];
}
class TestRuntime {
  public static tests: TestCase[] = [];
  public static suites: TestSuite[] = [new TestSuite()];
  public static results: TestResult[] = [];
  public static suite = "";
  public static skipSuite = false;
  public static onlySuite = false;
  public static focused = false;

  public static scope(title: string, fn: (this: TestContext) => void,
                      skipped = false, only = false): void {
    const parent = TestRuntime.suite;
    const parentSkip = TestRuntime.skipSuite;
    const parentOnly = TestRuntime.onlySuite;
    TestRuntime.suite = parent === "" ? title : parent + " " + title;
    TestRuntime.skipSuite = parentSkip || skipped;
    TestRuntime.onlySuite = parentOnly || only;
    if (only) { TestRuntime.focused = true; }
    TestRuntime.suites.push(new TestSuite());
    try {
      fn.call(new TestContext());
    } finally {
      TestRuntime.suites.pop();
      TestRuntime.suite = parent;
      TestRuntime.skipSuite = parentSkip;
      TestRuntime.onlySuite = parentOnly;
    }
  }
  public static register(title: string, fn: () => void, only = false, skipped = false): void {
    if (only) { TestRuntime.focused = true; }
    const fullTitle = TestRuntime.suite === "" ? title : TestRuntime.suite + " " + title;
    TestRuntime.tests.push(new TestCase(fullTitle, fn, only || TestRuntime.onlySuite,
                                      skipped || TestRuntime.skipSuite, TestRuntime.suites.slice()));
  }
  public static async run(): Promise<TestSummary> {
    const summary = new TestSummary();
    TestRuntime.results = [];
    const remaining = new Map<TestSuite, number>();
    const started = new Map<TestSuite, boolean>();
    const failures = new Map<TestSuite, unknown>();
    const tests = TestRuntime.tests.filter(test => !test.skipped && (!TestRuntime.focused || test.only));
    for (const test of tests) {
      for (const suite of test.suites) { remaining.set(suite, (remaining.get(suite) || 0) + 1); }
    }
    for (const test of tests) {
      let failed = false;
      let message = "";
      const entered: TestSuite[] = [];
      try {
        for (const suite of test.suites) {
          if (!started.has(suite)) {
            started.set(suite, true);
            try { for (const hook of suite.beforeAll) { await hook.call(new TestContext()); } }
            catch (error) { failures.set(suite, error); throw error; }
          }
          if (failures.has(suite)) { throw failures.get(suite); }
          entered.push(suite);
          for (const hook of suite.beforeEach) { await hook.call(new TestContext()); }
        }
        await test.body.call(new TestContext());
      } catch (error) { failed = true; message = String(error); }
      for (let i = entered.length - 1; i >= 0; i--) {
        try { for (const hook of entered[i].afterEach) { await hook.call(new TestContext()); } }
        catch (error) { if (!failed) { failed = true; message = String(error); } }
      }
      for (let i = test.suites.length - 1; i >= 0; i--) {
        const suite = test.suites[i];
        const count = (remaining.get(suite) || 0) - 1;
        remaining.set(suite, count);
        if (count === 0 && started.has(suite)) {
          try { for (const hook of suite.afterAll) { await hook.call(new TestContext()); } }
          catch (error) { if (!failed) { failed = true; message = String(error); } }
        }
      }
      if (failed) {
        summary.failed++;
        summary.failures.push(new TestFailure(test.title, message));
      } else { summary.passed++; }
      TestRuntime.results.push(new TestResult(test.title, !failed, message));
    }
    return summary;
  }
}
function describe(title: string, fn: (this: TestContext) => void): void {
  TestRuntime.scope(title, fn);
}
namespace describe {
  export function skip(title: string, fn: (this: TestContext) => void): void {
    TestRuntime.scope(title, fn, true);
  }
  export function only(title: string, fn: (this: TestContext) => void): void {
    TestRuntime.scope(title, fn, false, true);
  }
}
function it(title: string, fn: () => void): void { TestRuntime.register(title, fn); }
namespace it {
  export function only(title: string, fn: () => void): void { TestRuntime.register(title, fn, true); }
  export function skip(title: string, fn: () => void): void { TestRuntime.register(title, fn, false, true); }
}

function before(fn: () => void): void { TestRuntime.suites[TestRuntime.suites.length - 1].beforeAll.push(fn); }
function after(fn: () => void): void { TestRuntime.suites[TestRuntime.suites.length - 1].afterAll.push(fn); }
function beforeEach(fn: () => void): void { TestRuntime.suites[TestRuntime.suites.length - 1].beforeEach.push(fn); }
function afterEach(fn: () => void): void { TestRuntime.suites[TestRuntime.suites.length - 1].afterEach.push(fn); }
