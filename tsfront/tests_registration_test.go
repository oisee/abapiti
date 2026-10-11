package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChaiAndInitializationGo(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	sources["test/probe.ts"] = `import {expect} from "../runtime/chai";
 class Item { public value = 1; }
 const unsupported = new Date();
 function checkInstance(value: any, constructor: any): void { expect(value).to.be.instanceof(constructor); }
 function rows(tests: {abap: string, rel: Item | string, lang?: string}[]): void {
  for (const test of tests) { it(test.abap, () => {}); }
 }
 const names = ["contextual map row"];
 rows(names.map(abap => ({abap, rel: new Item()})));
 function unavailable(): number { throw new Error("runtime initializer failed"); }
 const guarded = unavailable();
 describe("recovered", () => {
  it("primitives", () => {
   expect(1).to.equal(1); expect(1).to.not.equal("1");
   expect("abc").to.equal("abc"); expect(true).to.equal(true);
   expect(false).to.not.equal(0); expect(undefined).to.equal(undefined);
   expect(null).to.equal(null); expect(null).to.not.equal(undefined);
  });
  it("references", () => {
   const a = new Item(); const b = new Item();
   expect(a).to.equal(a); expect(a).to.not.equal(b);
   expect(a).to.be.instanceof(Item); checkInstance(a, Item); expect(1).to.not.be.instanceof(Item);
   expect(undefined).to.not.be.instanceof(Item); expect(null).to.not.be.instanceof(Item);
  });
  it("guarded binding", () => expect(guarded).to.equal(0));
  it("ordinary failure", () => expect(new Item()).to.equal(new Item()));
 });
 throw new Error("init failure payload");
 describe("after throw", () => it("registered", () => expect(true).to.equal(true)));
 describe("async suite", async () => { it("async child", async () => {}); it("sync child", () => {}); });
 `
	var rows []map[string]string
	order := []string{"runtime/mocha.ts", "runtime/chai.ts", "test/probe.ts"}
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
	raw, _ := json.Marshal(map[string]any{"upstreamPin": RegistryUpstreamPin, "roots": []string{"test/probe.ts"}, "initialization": order, "sources": rows})
	if err := os.WriteFile(filepath.Join(dir, "closure.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["runtime/*.ts","test/*.ts"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	l, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, l)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var result struct {
		Passed, Failed, Trapped, NotRun, AsyncPending int
		Results                                       []json.RawMessage
		Initialization                                []struct{ File, Action, Message, Status string }
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if code != 1 || result.Passed != 6 || result.Failed != 1 || result.Trapped != 1 || result.NotRun != 0 || result.AsyncPending != 0 || len(result.Results) != 8 || len(result.Initialization) != 3 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if result.Initialization[0].File != "test/probe.ts" || !strings.Contains(result.Initialization[0].Message, "Date") || !strings.Contains(result.Initialization[1].Message, "runtime initializer failed") || !strings.Contains(result.Initialization[2].Message, "init failure payload") {
		t.Fatal(result.Initialization)
	}
}
