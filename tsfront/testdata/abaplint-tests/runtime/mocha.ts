// A deliberately synchronous test runtime. No Node APIs, timers or promises.
class TestContext {
  public timeout(_milliseconds: number): void {}
}
class TestFailure {
  public constructor(public title: string, public message: string) {}
}
class TestResult {
  public constructor(public title: string, public passed: boolean, public message: string) {}
}
class TestCase {
  public constructor(public title: string, public body: () => void,
                     public only: boolean, public skipped: boolean) {}
}
class TestSummary {
  public passed = 0;
  public failed = 0;
  public failures: TestFailure[] = [];
}
class TestRuntime {
  public static tests: TestCase[] = [];
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
    try {
      fn.call(new TestContext());
    } finally {
      TestRuntime.suite = parent;
      TestRuntime.skipSuite = parentSkip;
      TestRuntime.onlySuite = parentOnly;
    }
  }
  public static register(title: string, fn: () => void, only = false, skipped = false): void {
    if (only) { TestRuntime.focused = true; }
    const fullTitle = TestRuntime.suite === "" ? title : TestRuntime.suite + " " + title;
    TestRuntime.tests.push(new TestCase(fullTitle, fn, only || TestRuntime.onlySuite,
                                      skipped || TestRuntime.skipSuite));
  }
  public static run(): TestSummary {
    const summary = new TestSummary();
    TestRuntime.results = [];
    const focused = TestRuntime.focused;
    for (const test of TestRuntime.tests) {
      if (test.skipped || (focused && !test.only)) {
        continue;
      }
      try {
        test.body();
        summary.passed++;
        TestRuntime.results.push(new TestResult(test.title, true, ""));
      } catch (error) {
        const message = String(error);
        summary.failed++;
        summary.failures.push(new TestFailure(test.title, message));
        TestRuntime.results.push(new TestResult(test.title, false, message));
      }
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
