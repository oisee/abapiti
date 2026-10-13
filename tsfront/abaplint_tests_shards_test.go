package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAbaplintTestShardsOSGEmission(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	dir := t.TempDir()
	// OSG 16c024c8 accepts newline PCRE (#726); the reviewed DDL exec pattern's
	// leading \b lowers to a negative lookbehind that gogen still refuses with
	// a host NOT_COMPILED dump: exercise init/body recovery without editing ABAP.
	sources := map[string]string{
		"test/init_a.ts": `(/\bdefine\s+(?:\w+\s+)*?behavior\s+for\s+([\w/]+)(?:\s+alias\s+(\w+))?/gi).exec("define behavior for foo");`,
		"test/shared.ts": `it("duplicate",()=>{});`,
		"test/a.ts":      `import {expect} from "../runtime/chai"; import "./shared"; it("duplicate",()=>{const r=/foo/; r.exec("foo")}); it("assertion",()=>expect.fail("keep FAIL")); it("dump",()=>{(/\bdefine\s+(?:\w+\s+)*?behavior\s+for\s+([\w/]+)(?:\s+alias\s+(\w+))?/gi).exec("define behavior for foo")}); it("after",()=>{});`,
		"test/b.ts":      `import "./shared"; it("duplicate",()=>{const r=/foo/; r.exec("foo")});`,
	}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	order := []string{"runtime/mocha.ts", "runtime/chai.ts", "test/init_a.ts", "test/shared.ts", "test/a.ts", "test/b.ts"}
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
	shards := []map[string]any{
		{"name": "a", "initialization": order[:5]},
		{"name": "b", "initialization": []string{"runtime/mocha.ts", "runtime/chai.ts", "test/shared.ts", "test/b.ts"}},
	}
	manifest, _ := json.Marshal(map[string]any{"upstreamPin": RegistryUpstreamPin, "roots": []string{"test/a.ts", "test/b.ts"}, "initialization": order, "sources": rows, "shards": shards})
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
	files, err := emitABAPTestsOSG(t, dir, lowering)
	if err != nil {
		t.Fatal(err)
	}
	runner := files["zcl_abapiti_test_runner.clas.abap"]
	for _, key := range []string{"shard TYPE string", "CASE shard.", "WHEN `a`.", "WHEN `b`.", "WHEN OTHERS.", "RETURN."} {
		if !strings.Contains(runner, key) {
			t.Fatalf("missing selection boundary %q", key)
		}
	}
	a := strings.Split(strings.Split(runner, "WHEN `a`.\n")[1], "WHEN `b`.")[0]
	b := strings.Split(strings.Split(runner, "WHEN `b`.\n")[1], "WHEN OTHERS.")[0]
	if strings.Contains(a, "test/b.ts") || strings.Contains(b, "test/a.ts") || strings.Contains(b, "test/init_a.ts") || !strings.Contains(a, `"index":1,`) || !strings.Contains(b, `"index":1,`) {
		t.Fatal("shard initialization leaked or indices did not restart")
	}
	osg := os.Getenv("ABAPITI_OSG_HOME")
	if osg == "" {
		return
	}
	out := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	build := t.TempDir()
	cmd := exec.Command("node", "--expose-gc", "../tools/abaplint-tests-osg.mjs", out, build, osg)
	cmd.Env = append(os.Environ(), "ABAPITI_OSG_BUILD_ONLY=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shared build: %v\n%s", err, output)
	}
	// One binary, alternating selections, independent resumptions. Duplicated
	// title strings retain their multiplicity; borrowed registrations run once.
	// An earlier genuine FAIL survives both dumps; neither skips nor indices
	// leak from a into b or into the next fresh run of a.
	for _, shard := range []string{"a", "b", "a", "b"} {
		cmd := exec.Command("node", "../tools/osg-runner-execute.mjs", filepath.Join(build, "abaplint-tests-osg"), filepath.Join(out, "names.json"), t.TempDir(), shard)
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		var report struct {
			Passed, Failed, Trapped int
			Results                 []struct{ Title, Status, Message string }
			Initialization          []struct{ File, Status string }
		}
		if decode := json.Unmarshal([]byte(stdout.String()), &report); decode != nil {
			t.Fatalf("shard %s: %v / %v\n%s\n%s", shard, err, decode, stdout.String(), stderr.String())
		}
		titles := []string{}
		for _, r := range report.Results {
			titles = append(titles, r.Title)
		}
		want := []string{"duplicate", "duplicate"}
		wantFailed := 0
		if shard == "a" {
			wantFailed = 1
			want = append(want, "assertion", "dump", "after")
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || report.Passed != 3 || report.Trapped != 1 || len(report.Initialization) != 1 || report.Initialization[0].File != "test/init_a.ts" || report.Initialization[0].Status != "trapped" || strings.Count(stderr.String(), "OSG harness refused:") != 2 {
				t.Fatalf("lost shard-local host recovery: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
		} else if err != nil || report.Passed != 2 || report.Trapped != 0 || len(report.Initialization) != 0 {
			t.Fatalf("shard b contaminated: %v\n%s", err, stdout.String())
		}
		if report.Failed != wantFailed || !reflect.DeepEqual(titles, want) {
			t.Fatalf("shard %s registration multiplicity: %v, want %v", shard, titles, want)
		}
		if shard == "a" && (report.Results[2].Status != "failed" || report.Results[2].Message != "keep FAIL" || report.Results[3].Status != "trapped" || report.Results[4].Status != "passed") {
			t.Fatalf("FAIL/dump/following pass changed: %s", stdout.String())
		}
	}
	cmd = exec.Command("node", "../tools/osg-runner-execute.mjs", filepath.Join(build, "abaplint-tests-osg"), filepath.Join(out, "names.json"), t.TempDir(), "unknown")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "without a resumable checkpoint") {
		t.Fatalf("unknown shard manufactured results: %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
}
