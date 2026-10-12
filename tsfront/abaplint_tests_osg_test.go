package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestABAPTestGapCountsSyntheticOccurrences(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	unsupported := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.sha1Hex", Type: hir.T(hir.String), X: hir.L(hir.T(hir.String), "a")}}
	}
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(unsupported(), unsupported(),
		&hir.Stmt{Kind: hir.ExprStmt, X: hir.L(hir.T(hir.Number), math.Inf(1))})}
	lowering := &RegistryLowering{Prog: &hir.Program{Classes: []*hir.Class{{Name: "Host", Methods: []*hir.Method{m}}}}}
	gap := MeasureABAPTestGap(lowering)
	if len(gap.UnsupportedCatalogue) != 26 || len(gap.Runtime) != 1 || gap.Runtime[0].Sites != 2 || gap.NonfiniteLiteralSites != 1 {
		t.Fatalf("lost synthetic occurrences or ABAP limits: %+v", gap)
	}
	if len(gap.Refusals) != 1 || !strings.Contains(gap.Refusals[0].Reason, "non-finite Number") {
		t.Fatalf("lost nonfinite emitter refusal: %+v", gap.Refusals)
	}
}

func TestABAPTestRefusalsSyntheticOwnership(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	m := &hir.Method{Node: hir.Node{ID: 1}, Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "number.ceil", Type: hir.T(hir.Number), X: hir.L(hir.T(hir.Number), 1.0)}})}
	lowering := &RegistryLowering{Prog: &hir.Program{Classes: []*hir.Class{{Name: "Host", Methods: []*hir.Method{m}}}}}
	if err := TrapABAPTestRefusals(lowering); err != nil {
		t.Fatal(err)
	}
	if !isFailureBody(m.Body) || !strings.Contains(m.Body.List[0].Name, "number.ceil") {
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
	dir := t.TempDir()
	sources := map[string]string{"test/host.ts": `import {expect} from "../runtime/chai";
it("pass",()=>{});it("assertion",()=>{expect(1).equal(2)});it("refusal",()=>{new Date()});it("intentional fail",()=>expect.fail("intentional failure"));`}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	order := []string{"runtime/mocha.ts", "runtime/chai.ts", "test/host.ts"}
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
	manifest, _ := json.Marshal(map[string]any{"upstreamPin": RegistryUpstreamPin, "roots": []string{"test/host.ts"}, "initialization": order, "sources": rows})
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
	if len(gap.Runtime) == 0 {
		t.Fatal("missing chai operation gap")
	}
	files, err := EmitAbaplintTestsOSG(dir, lowering)
	if err != nil {
		t.Fatal(err)
	}
	runner := files["zcl_abapiti_test_runner.clas.abap"]
	for _, key := range []string{`"asyncPending"`, `"passed"`, `"failed"`, `"trapped"`, `"notRun"`, `"results"`, `"initialization"`, "CATCH z_exception_te", "source_location", "hook execution is not yet emitted", "settled->rejection", "escape( val = value"} {
		if !strings.Contains(runner, key) {
			t.Fatalf("runner missing %s", key)
		}
	}
	out := os.Getenv("ABAPITI_TEST_OUT")
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
		var summary struct {
			Passed, Failed, Trapped int
			Results                 []struct{ Title, Status, Message string }
		}
		if err := json.Unmarshal([]byte(stdout.String()), &summary); err != nil {
			t.Fatal(err)
		}
		if summary.Passed != 1 || summary.Failed != 1 || summary.Trapped != 2 {
			t.Fatalf("OSG misclassification: %s", stdout.String())
		}
		last := summary.Results[len(summary.Results)-1]
		if last.Title != "intentional fail" || last.Status != "failed" || last.Message != "intentional failure" {
			t.Fatalf("settled assertion was lost: %s", stdout.String())
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
