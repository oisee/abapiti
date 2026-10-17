package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestABAPTestGapCountsSyntheticOccurrences(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	unsupported := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.parseFloat", Type: hir.T(hir.Optional, hir.T(hir.Number)), X: hir.L(hir.T(hir.String), "a")}}
	}
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(unsupported(), unsupported(),
		&hir.Stmt{Kind: hir.ExprStmt, X: hir.L(hir.T(hir.Number), math.Inf(1))})}
	lowering := &RegistryLowering{Prog: &hir.Program{Classes: []*hir.Class{{Name: "Host", Methods: []*hir.Method{m}}}}}
	gap := MeasureABAPTestGap(lowering)
	// Classview, Chai, JSON5 and the checked string/XML operations are admitted.
	wantCatalogue := []string{"array.matchGroups", "array.spread0", "arrayLike.length", "clock.epochMilliseconds", "dynamic.isMap", "dynamic.isSet", "dynamic.spread", "dynamic.toNumber", "dynamic.values", "fs.readText", "fs.readdirEntries", "fs.readdirNames", "host.getenv", "host.stderrWrite", "json.stringifyReplacer", "map.insertions", "map.markJS", "map.view", "number.abs", "number.hex16", "number.isFinite", "number.isInteger", "number.isSafeInteger", "number.strictIsNaN", "path.basename", "path.join", "regexp.clone", "regexp.flags", "regexp.matchAllNext", "set.iterationValues", "string.codePoints", "string.padEnd", "string.parseFloat", "string.relativeAt", "string.sha256Hex", "union.admit", "union.check", "union.project", "union.read", "union.tag", "union.widen", "util.isDeepStrictEqual", "weak.lookupKey", "weak.writeKey"}
	if !slices.Equal(gap.UnsupportedCatalogue, wantCatalogue) || len(gap.Runtime) != 1 || gap.Runtime[0].Sites != 2 || gap.NonfiniteLiteralSites != 1 {
		t.Fatalf("lost synthetic occurrences or ABAP limits: %+v", gap)
	}
	for _, op := range gap.UnsupportedCatalogue {
		if op == "json.stringifySubset" || op == "json.indent" || op == "object.checkedClassView" || op == "classvalue.newArgs" || op == "string.sha1Hex" || op == "string.splitRegex" || op == "string.padStart" || op == "string.parseFloatNumber" || op == "xml.parseWithErrors" {
			t.Fatalf("implemented operation still catalogue-refused: %s", op)
		}
	}
	if len(gap.Refusals) != 1 || !strings.Contains(gap.Refusals[0].Reason, "non-finite Number") {
		t.Fatalf("lost nonfinite emitter refusal: %+v", gap.Refusals)
	}
}

func TestABAPTestRefusalsSyntheticOwnership(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	m := &hir.Method{Node: hir.Node{ID: 1}, Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.parseFloat", Type: hir.T(hir.Optional, hir.T(hir.Number)), X: hir.L(hir.T(hir.String), "a")}})}
	lowering := &RegistryLowering{Prog: &hir.Program{Classes: []*hir.Class{{Name: "Host", Methods: []*hir.Method{m}}}}}
	if err := TrapABAPTestRefusals(lowering); err != nil {
		t.Fatal(err)
	}
	if !isFailureBody(m.Body) || !strings.Contains(m.Body.List[0].Name, "string.parseFloat") {
		t.Fatalf("did not trap ID-zero owning method: %#v", m.Body)
	}
}

func TestABAPTestRefusalsDoNotCrossSyntheticOwners(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	bad := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.parseIntNumber", Type: hir.T(hir.Number), X: hir.L(hir.T(hir.String), "not a number")}})}
	good := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B()}
	lowering := &RegistryLowering{Prog: &hir.Program{Classes: []*hir.Class{{Name: "Bad", Methods: []*hir.Method{bad}}, {Name: "Good", Methods: []*hir.Method{good}}}}}
	if err := TrapABAPTestRefusals(lowering); err != nil {
		t.Fatal(err)
	}
	if !isFailureBody(bad.Body) || isFailureBody(good.Body) {
		t.Fatal("a synthetic refusal crossed class ownership")
	}
}

func TestAbaplintTestHostOSGEmission(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	dir := t.TempDir()
	// OSG 16c024c8 accepts newline PCRE and ASCII word boundaries (#726).
	// An admitted sticky regex with lastIndex=1001 emits a cursor repeat
	// [\s\S]{1001}, beyond RE2's 1000-repeat limit. compilePCRE in
	// gogen/go/abap/find.go refuses it with host NOT_COMPILED, not a
	// catchable ABAP exception: exercise init and body restarts.
	sources := map[string]string{
		"test/init_dump.ts": `const initPattern = /a/y; initPattern.lastIndex = 1001; initPattern.exec("a".repeat(1002));`,
		"test/host.ts": `import {expect} from "../runtime/chai";
it("pass",()=>{});it("assertion",()=>{expect(1).equal(2)});it("refusal",()=>{new Date()});it("intentional fail",()=>expect.fail("intentional failure"));
it("host dump",()=>{const r = /a/y; r.lastIndex = 1001; r.exec("a".repeat(1002))});it("after host dump",()=>{});
it("JS whitespace on multiline text",()=>{expect("a\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeffb\u0085\u180ec".replace(/\s+/g,"_")).equal("a_b\u0085\u180ec")});it("JS ASCII word and digit",()=>{expect("a\nA_0é١\ufeff".replace(/[\w\d]+/g,"_")).equal("_\n_é١\ufeff")});
it("native host refusal",()=>{const r = /a/y; r.lastIndex = 1001; r.exec("a".repeat(1002))});it("CDS normalization",()=>{expect("a\n\u00a0b\vc😀".replace(/\u00a0/g," ").replace(/\u000b/g," ")).equal("a\n b c😀")});it("native exception identity",()=>{throw new Error("ordinary throw")});it("checked absolute anchors",()=>{const r = new RegExp("^foo$",""); expect(r.exec("foo") === null).equal(false); expect(r.exec("xfoo") === null).equal(true); expect(r.exec("foo\nbar") === null).equal(true)});it("CDS quoted grammar",()=>{const r = /^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$/; expect(r.exec("'he''llo'") === null).equal(false); expect(r.exec("''junk") === null).equal(true)});it("array storage length",()=>{const a = new Array<number>(2); expect(a.length).equal(2)});`,
		"test/native-init.ts": `const r = (()=>{const regex = /a/y; regex.lastIndex = 1001; regex.exec("a".repeat(1002)); return regex})(); r.exec("a");`,
	}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	order := []string{"runtime/mocha.ts", "runtime/chai.ts", "test/init_dump.ts", "test/native-init.ts", "test/host.ts"}
	rows := []map[string]string{}
	for _, name := range order {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(sources[name]), 0644); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, map[string]string{"file": name, "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(sources[name])))})
	}
	manifest, _ := json.Marshal(map[string]any{"upstreamPin": RegistryUpstreamPin, "roots": []string{"test/init_dump.ts", "test/native-init.ts", "test/host.ts"}, "initialization": order, "sources": rows})
	if err := os.WriteFile(filepath.Join(dir, "closure.json"), manifest, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["runtime/*.ts","test/*.ts"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	lowering, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	gap := MeasureABAPTestGap(lowering)
	for _, row := range gap.Runtime {
		if strings.HasPrefix(row.Reason, "chai.") {
			t.Fatal("Chai must emit rather than refuse its entire owning body", row)
		}
	}
	files, err := emitABAPTestsOSG(t, dir, lowering)
	if err != nil {
		t.Fatal(err)
	}
	runner := files["zcl_abapiti_test_runner.clas.abap"]
	for _, key := range []string{`"asyncPending"`, `"passed"`, `"failed"`, `"trapped"`, `"notRun"`, `"results"`, `"initialization"`, "CATCH z_exception_te", "source_location", "hook execution is not yet emitted", "settled->rejection", "escape( val = value", "start_index TYPE i", "skip_init TYPE string", `"kind":"plan"`, `"kind":"registry"`, `"kind":"init"`, `"kind":"init-done"`, `"kind":"test"`, `"kind":"result"`, "test_index < start_index", "METHOD host_body.", "METHOD host_init_1.", "status = 'trapped' message = host_message_1", "IMPORTING settled = settled message = message"} {
		if !strings.Contains(runner, key) {
			t.Fatalf("runner missing %s", key)
		}
	}
	// Every boundary, including the forced runtime initializer, must have the
	// same stable one-based plan, checkpoint and skip index.
	boundaries := regexp.MustCompile(`METHOD host_init_(\d+)\.`).FindAllStringSubmatch(runner, -1)
	if len(boundaries) == 0 || !strings.Contains(runner, fmt.Sprintf(`"initializations":%d`, len(boundaries))) {
		t.Fatal("initialization plan does not count every host boundary")
	}
	for i, boundary := range boundaries {
		index, err := strconv.Atoi(boundary[1])
		if err != nil || index != i+1 {
			t.Fatalf("unstable host initialization index: %q", boundary[1])
		}
		for _, key := range []string{fmt.Sprintf("IF skip_init NS ',%d,'.", index), fmt.Sprintf(`"kind":"init","index":%d,`, index), fmt.Sprintf("DATA(host_message_%d) = host_init_%d( ).", index, index)} {
			if !strings.Contains(runner, key) {
				t.Fatalf("boundary %d missing skip/checkpoint/call: %s", index, key)
			}
		}
	}
	out := fixtureOutputDir(t)
	if osg := os.Getenv("ABAPITI_OSG_HOME"); osg != "" {
		if out == "" {
			out = t.TempDir()
		}
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, src := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
		run := exec.Command("node", "../tools/abaplint-tests-osg.mjs", out, t.TempDir(), osg)
		var stdout, stderr strings.Builder
		run.Stdout = &stdout
		run.Stderr = &stderr
		err := run.Run()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("OSG fixture exit: %v\n%s\n%s", err, stdout.String(), stderr.String())
		}
		if dumps := strings.Count(stderr.String(), "OSG harness refused:"); dumps != 4 {
			t.Fatalf("wanted four real host dumps (two init, two body), got %d:\n%s\n%s", dumps, stdout.String(), stderr.String())
		}
		t.Log("4 real host dumps; 5 binary invocations; 14 registrations reported")
		var summary struct {
			Passed, Failed, Trapped int
			Results                 []struct{ Title, Status, Message string }
			Initialization          []struct{ File, Status, Message string }
		}
		if err := json.Unmarshal([]byte(stdout.String()), &summary); err != nil {
			t.Fatalf("OSG fixture returned no valid report: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
		}
		// Recovery preserves host traps; hole-aware allocation now permits
		// the sized-array length observation without a blanket Chai refusal.
		if summary.Passed != 8 || summary.Failed != 2 || summary.Trapped != 4 {
			t.Fatalf("OSG misclassification: %s", stdout.String())
		}
		wantTitles := []string{"pass", "assertion", "refusal", "intentional fail", "host dump", "after host dump", "JS whitespace on multiline text", "JS ASCII word and digit", "native host refusal", "CDS normalization", "native exception identity", "checked absolute anchors", "CDS quoted grammar", "array storage length"}
		if len(summary.Results) != len(wantTitles) {
			t.Fatalf("lost or duplicated registrations: %s", stdout.String())
		}
		for i, title := range wantTitles {
			if summary.Results[i].Title != title {
				t.Fatalf("registration %d: want %q, got %q", i+1, title, summary.Results[i].Title)
			}
		}
		for _, i := range []int{6, 7} {
			if summary.Results[i].Status != "passed" {
				t.Fatalf("16c024c8 newline replacement regressed: %s", stdout.String())
			}
		}
		if summary.Results[1].Title != "assertion" || summary.Results[1].Status != "failed" || summary.Results[1].Message != "assertion failed: expected 1 to equal 2" || summary.Results[2].Title != "refusal" || summary.Results[2].Status != "trapped" {
			t.Fatalf("Chai assertion/trap boundary was lost: %s", stdout.String())
		}
		intentional := summary.Results[3]
		if intentional.Title != "intentional fail" || intentional.Status != "failed" || intentional.Message != "intentional failure" {
			t.Fatalf("settled assertion was lost: %s", stdout.String())
		}
		if len(summary.Initialization) != 3 || summary.Initialization[1].File != "test/native-init.ts" || summary.Initialization[1].Status != "trapped" || !strings.Contains(summary.Initialization[1].Message, "NOT_COMPILED in FIND PCRE") || !strings.Contains(summary.Initialization[1].Message, "invalid repeat count: `{1001}`") || !strings.Contains(summary.Initialization[1].Message, "find.go:") {
			t.Fatalf("native initialization refusal was lost: %s", stdout.String())
		}
		if summary.Initialization[2].File != "test/native-init.ts" || summary.Initialization[2].Status != "trapped" || !strings.Contains(summary.Initialization[2].Message, "was not initialized") {
			t.Fatalf("subsequent real action did not retain its binding refusal: %s", stdout.String())
		}
		native := summary.Results[8]
		if native.Title != "native host refusal" || native.Status != "trapped" || !strings.Contains(native.Message, "NOT_COMPILED in FIND PCRE") || !strings.Contains(native.Message, "invalid repeat count: `{1001}`") || !strings.Contains(native.Message, "find.go:") {
			t.Fatalf("native refusal escaped its body or became a pass: %s", stdout.String())
		}
		generic := summary.Results[10]
		if generic.Title != "native exception identity" || generic.Status != "trapped" || !strings.Contains(generic.Message, "[exception.") {
			t.Fatalf("native exception identity was lost or relabeled: %s", stdout.String())
		}
		holes := summary.Results[len(summary.Results)-1]
		if holes.Title != "array storage length" || holes.Status != "passed" {
			t.Fatalf("hole-aware array length was refused: %s", stdout.String())
		}
		hostDump, last := summary.Results[4], summary.Results[5]
		if hostDump.Title != "host dump" || hostDump.Status != "trapped" || !strings.Contains(hostDump.Message, "NOT_COMPILED in FIND PCRE") || !strings.Contains(hostDump.Message, "invalid repeat count: `{1001}`") || !strings.Contains(hostDump.Message, "find.go:") || last.Title != "after host dump" || last.Status != "passed" {
			t.Fatalf("host dump did not resume with location: %s", stdout.String())
		}
		if len(summary.Initialization) != 3 || summary.Initialization[0].File != "test/init_dump.ts" || summary.Initialization[0].Status != "trapped" || !strings.Contains(summary.Initialization[0].Message, "NOT_COMPILED in FIND PCRE") || !strings.Contains(summary.Initialization[0].Message, "invalid repeat count: `{1001}`") {
			t.Fatalf("initialization dump did not survive: %s", stdout.String())
		}
	}
	if out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, src := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
