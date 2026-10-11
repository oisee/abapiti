package tsfront

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestAbaplintTestRuntimePrimitivesGo(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "")
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Holder {
  public n = 0;
  public body: () => void;
  constructor() { this.body = () => { this.n++; }; }
  get value(): number { this.n++; return this.n; }
 }
 export class Probe {
  private n = 0;
  private next(): string | undefined { this.n++; return this.n === 1 ? undefined : "x"; }
  private callable(fn?: () => void): boolean { return typeof fn === "function"; }
  run(): string {
   const h = new Holder(); h.body();
   if (!this.callable(() => {}) || this.callable()) { throw new Error("typeof thunk"); }
   const a = h.value; const b = h.value;
   if (a !== 2 || b !== 3) { throw new Error("getter/thunk"); }
   const absent = typeof this.next() === "undefined";
   const present = typeof this.next() === "string";
   if (!absent || !present || this.n !== 2) { throw new Error("typeof evaluation"); }
   const missing: string | undefined = undefined;
   if (typeof missing !== "undefined") { throw new Error("typeof absence"); }
   const empty: string | undefined = "";
   if (typeof empty !== "string") { throw new Error("typeof empty"); }
   return "ab".repeat(2.8) + ":" + "x".repeat() + String(17) + ":" + String(undefined);
  }
 }`}, []string{"probe.ts"})
	if got := runGoHIR(t, p, "fmt.Println("+hir.NewNames().Get("new.probe.ts.Probe")+"()."+hir.NewNames().Get("member.run")+"().String())"); got != "abab:17:undefined\n" {
		t.Fatal(got)
	}
}

func TestAbaplintRepeatABAPSupported(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "")
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe { run(): string { return "x".repeat(2); } }`}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatalf("ABAP repeat support from #93: %v", err)
	}
}

func TestAbaplintTestsNilRuntimeReceiver(t *testing.T) {
	// Incomplete lowering can leave an operation with a missing receiver.
	// Numeric passes must not crash before the verifier reports the error.
	for _, assume := range []string{"", "1"} {
		t.Run("assume="+assume, func(t *testing.T) {
			t.Setenv("ABAPITI_ASSUME_INT", assume)
			x := &hir.Expr{Kind: hir.RuntimeOp, Op: "regexp.new", Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(hir.T(hir.String), "a.c"), hir.L(hir.T(hir.String), "i")}}
			p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.RegExp), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: x})}}}}}
			l := &lowerer{out: p, floatSites: map[string]bool{}}
			l.inferNumberRanges()
			if assume == "1" {
				l.assumeIntegerTypes()
			}
			if errs := hir.Verify(p); len(errs) == 0 || !strings.Contains(errs[0].Error(), "missing expression") {
				t.Fatalf("missing receiver was not diagnosed: %v", errs)
			}
			if x.Op != "regexp.new" || x.X != nil || x.Type.Kind != hir.RegExp {
				t.Fatal("lost receiverless operation")
			}
		})
	}
}

func TestAbaplintTestsNilRuntimeArgument(t *testing.T) {
	x := &hir.Expr{Kind: hir.RuntimeOp, Op: "string.repeat", Type: hir.T(hir.String), X: hir.L(hir.T(hir.String), "x"), Args: []*hir.Expr{nil}}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: x})}}}}}
	l := &lowerer{out: p, floatSites: map[string]bool{}}
	l.inferNumberRanges()
	l.assumeIntegerTypes()
	if errs := hir.Verify(p); len(errs) == 0 || !strings.Contains(errs[0].Error(), "missing expression") {
		t.Fatalf("missing argument was not diagnosed: %v", errs)
	}
}

func TestAbaplintGettersGo(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "")
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Base { get label(): string { return "base"; } }
 export class Derived extends Base { get label(): string { return super.label + "!"; } }
 export class Probe {
  static get label(): string { return "static"; }
  run(): string { return new Derived().label + ":" + Probe.label; }
 }`}, []string{"probe.ts"})
	if got := runGoHIR(t, p, "fmt.Println("+hir.NewNames().Get("new.probe.ts.Probe")+"()."+hir.NewNames().Get("member.run")+"().String())"); got != "base!:static\n" {
		t.Fatal(got)
	}
}

func runAbaplintTestModule(t *testing.T, files map[string]string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(dir, "tests-go")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("test module build: %v\n%s", err, output)
	}
	run := exec.Command(binary)
	run.Dir = dir
	var stdout, stderr bytes.Buffer
	run.Stdout, run.Stderr = &stdout, &stderr
	code := 0
	if err := run.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return stdout.String(), stderr.String(), code
}

// Exercises the generated main, module initialization, JSON casing, per-test
// records and failure exit status. This is synthetic host glue, not abaplint.
func TestAbaplintTestHostGo(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"runtime/mocha.ts": `
 class FancyError extends Error {}
 class TestContext { timeout(_milliseconds: number): void {} }
 function describe(_title: string, fn: (this: TestContext) => void): void { fn.call(new TestContext()); }
 function registerContext(): void { describe("context", function() { this.timeout(1); if (this !== this) { throw new Error("unstable this context"); } register("context pass", () => {}); }); }
 class TestFailure { constructor(public title: string, public message: string) {} }
 class TestResult { constructor(public title: string, public passed: boolean, public message: string) {} }
 class TestCase { constructor(public title: string, public body: () => void, public only=false, public skipped=false) {} }
 class TestSummary { public passed=0; public failed=0; public failures: TestFailure[]=[]; public fraction=0.5; }
 class TestRuntime {
  public static tests: TestCase[]=[];
  public static focused=false;
  public static iteration=0;
  public static results: TestResult[]=[];
  public static add(title: string, body: () => void): void { TestRuntime.tests.push(new TestCase(title,body)); }
  public static run(): TestSummary {
   const s=new TestSummary();
   for(const test of TestRuntime.tests) {
    try { test.body(); s.passed++; TestRuntime.results.push(new TestResult(test.title,true,"")); }
    catch(error) { s.failed++; s.failures.push(new TestFailure(test.title,"failed")); TestRuntime.results.push(new TestResult(test.title,false,"failed")); }
   }
   return s;
  }
 }
 function register(title: string, body: () => void): void { TestRuntime.add(title,body); }
 function registerIterations(): void { for (const value of ["a", "b"]) { register("iteration " + value, () => { TestRuntime.iteration++; if ((TestRuntime.iteration === 1 && value !== "a") || (TestRuntime.iteration === 2 && value !== "b")) { throw new Error("stale for-of capture"); } }); } }
 `,
		"test/host.ts": `register("host pass", () => {}); register("host fail", () => { throw new Error("deliberate"); }); register("host trap", () => { const date = new Date(); }); register("host async", async () => {}); register("host done", (done: () => void) => { done(); }); registerIterations(); registerContext();`,
	}
	writeTestClosure(t, dir, sources)
	t.Setenv("ABAPITI_ASSUME_INT", "1") // the harness must still select binary64
	lowering, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, lowering)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var summary struct{ Passed, Failed, Trapped, NotRun int }
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil || code != 1 || summary.Passed != 5 || summary.Failed != 1 || summary.Trapped != 1 || summary.NotRun != 1 {
		t.Fatalf("host returned code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, `"title":"host pass","passed":true`) || !strings.Contains(stderr, `"title":"host fail","passed":false,"message":"deliberate"`) {
		t.Fatal(stderr)
	}
	// A tampered source is refused before lowering.
	if err = os.WriteFile(filepath.Join(dir, "test/host.ts"), []byte("// changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = LowerAbaplintTests(dir); err == nil {
		t.Fatal("unverified source accepted")
	}
}

// This gate never replaces upstream test bodies or supplies a coverage manifest.
func TestAbaplintOwnTestsGo(t *testing.T) {
	dir := os.Getenv("ABAPLINT_TEST_CLOSURE")
	if dir == "" {
		t.Skip("set ABAPLINT_TEST_CLOSURE to tools/abaplint-tests-closure.mjs output")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "")
	lowering, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		evidence, err := lowering.Evidence(false)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range evidence {
			if err = os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	files, err := EmitAbaplintTests(dir, lowering)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	t.Log(strings.TrimSpace(stderr))
	t.Log(strings.TrimSpace(stdout))
	if code != 0 {
		t.Fatalf("test executable exited %d", code)
	}
	var summary struct{ Passed, Failed int }
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil || summary.Passed == 0 || summary.Failed != 0 {
		t.Fatalf("not a passing test summary: %s (%v)", stdout, err)
	}
}

func writeTestClosure(t *testing.T, dir string, sources map[string]string) {
	t.Helper()
	manifest := map[string]any{"upstreamPin": RegistryUpstreamPin, "roots": []string{"test/host.ts"}, "initialization": []string{"runtime/mocha.ts", "test/host.ts"}}
	rows := []map[string]string{}
	for _, name := range []string{"runtime/mocha.ts", "test/host.ts"} {
		source := sources[name]
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, map[string]string{"file": name, "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(source)))})
	}
	manifest["sources"] = rows
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "closure.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["runtime/*.ts","test/*.ts"]}`), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAbaplintAsyncCallbacksAndHooksGo(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{
		"runtime/mocha.ts": string(runtime),
		"test/host.ts": `
 class State { static value = ""; }
 class Thenable { then(): void {} }
 before(async function() { this.timeout(1); State.value = "root"; });
 describe("async", () => {
  before(async () => { State.value = State.value + ":before"; });
  beforeEach(async () => { State.value = State.value + ":each"; });
  afterEach(async () => { State.value = State.value + ":afterEach"; });
  after(async () => { if (State.value !== "root:before:each:afterEach:each:afterEach:each:afterEach:each:afterEach:each:afterEach") { throw new Error(State.value); } });
  it("fulfilled", async () => { await Promise.resolve(); });
  it("returned", () => Promise.resolve(7));
  it("rejected", () => Promise.reject(new Error("rejected")));
  it("thrown", async () => { throw new Error("thrown"); });
  it("pending", () => new Promise<void>(() => {}));
 });
 it("optional promise", () => { if (State.value === "root") { return Promise.reject("must not pass"); } });
 it("uncertified thenable", () => new Thenable());
 it("primitive rejection", () => Promise.reject("literal reason"));
 it("undefined rejection", () => Promise.reject());
 it("diagnostic-like rejection", () => Promise.reject(new Error("unsupported-async user rejection")));
 describe("setup", () => {
  before(async () => { throw new Error("setup rejected"); });
  it("first", () => { throw new Error("body must not run"); });
  it("second", async () => {});
 });
 describe("afterEach", () => {
  afterEach(async () => { throw new Error("cleanup rejected"); });
  it("test", async () => {});
 });
 describe("after", () => {
  after(() => Promise.reject(new Error("after rejected")));
  it("test", async () => {});
 });
 describe("pending hook", () => {
  before(() => new Promise<void>(() => {}));
  it("test", async () => {});
 });
 describe.skip("skip", () => {
  before(async () => { throw new Error("must not run"); });
  it("test", async () => {});
 });
 `,
	})
	lowering, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, lowering)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var summary struct {
		Passed, Failed, Trapped, NotRun, AsyncPending int
		Results                                       []struct{ Title, Status, Message string }
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil || code != 1 || summary.Passed != 2 || summary.Failed != 9 || summary.Trapped != 0 || summary.NotRun != 1 || summary.AsyncPending != 4 {
		t.Fatalf("host returned code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, result := range summary.Results {
		if result.Status == "failed" && strings.Contains(result.Message, "body must not run") {
			t.Fatal(result)
		}
	}
	for _, want := range []string{`"title":"async rejected","passed":false,"message":"rejected"`, `"title":"async thrown","passed":false,"message":"thrown"`, `"status":"async-pending"`, `"message":"setup rejected"`, `"message":"cleanup rejected"`, `"message":"after rejected"`, `"message":"literal reason"`, `"message":"undefined"`, `"message":"unsupported-async user rejection","status":"failed"`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("missing %s in %s", want, stderr)
		}
	}
}
