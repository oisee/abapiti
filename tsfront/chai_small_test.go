package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeChaiClosure(t *testing.T, dir string, sources map[string]string) {
	t.Helper()
	order := []string{"runtime/mocha.ts", "runtime/chai.ts", "test/probe.ts"}
	var rows []map[string]string
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
}

func TestChaiSmallNodeOracle(t *testing.T) {
	chai := upstreamChaiModule(t)
	if _, err := os.Stat(chai); err != nil {
		t.Skip("pinned upstream Chai unavailable")
	}
	probe := `class A {} class B extends A {}
 function instance(value:any,type:new()=>any,negated:boolean):boolean {
  try { const assertion=negated?check(value).not:check(value);assertion.instanceOf(type);return true; } catch(error){return false;}
 }
 function includes(a:any,b:any,negated:boolean):boolean {
  try { const assertion=negated?check(a).not:check(a);assertion.contain(b);return true; } catch(error){return false;}
 }
 function fail():string {try {check.fail("payload");}catch(error){return error.message;}return "did not throw";}
 class expect {static fail(s:string):string{return s;}}
 function lookalike():string {return expect.fail("user method");}
 function decisions():string {
 const item={x:1};const list=[item];const nested=[1];
 return includes([1,2],2,false)+":"+includes([1,2],3,false)+":"+includes([1,2],3,true)+":"+
 includes(list,item,false)+":"+includes(list,{x:1},false)+":"+includes([nested],nested,false)+":"+
 includes([undefined],undefined,false)+":"+includes([null],undefined,false)+":"+includes("abc","b",false)+":"+fail()+":"+lookalike()+":"+
 instance(new A(),A,false)+":"+instance(new B(),A,false)+":"+instance(new A(),B,false)+":"+instance(1,A,false)+":"+instance(null,A,true)+":"+instance(undefined,A,false);

 }`
	oracle := fmt.Sprintf("const {expect:check}=require(%q);\n", chai) + probe + "\nconsole.log(decisions());"
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]))`, oracle).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	sources := map[string]string{"test/probe.ts": "import {expect as check} from '../runtime/chai';\n" + probe + fmt.Sprintf(`
 it("oracle",()=>{const got=decisions();if(got!==%q){throw new Error("Node oracle: "+got);}});
 it("genuine fail remains failed",()=>{check.fail("payload");});`, strings.TrimSpace(string(want)))}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	dir := t.TempDir()
	writeChaiClosure(t, dir, sources)
	l, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, l)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed, Trapped, NotRun int
		Results                         []struct{ Status, Message string }
		Initialization                  []any
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 1 || got.Passed != 1 || got.Failed != 1 || got.Trapped != 0 || got.NotRun != 0 || len(got.Initialization) != 0 {
		t.Fatalf("code=%d %s\n%s", code, stdout, stderr)
	}
	if got.Results[1].Status != "failed" || !strings.Contains(got.Results[1].Message, "payload") {
		t.Fatal(got.Results)
	}
}

// Imported batch-E oracles must work with a pinned checkout outside one home.
func upstreamChaiModule(t *testing.T) string {
	t.Helper()
	root := os.Getenv("ABAPITI_TESTS_UPSTREAM")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		root = filepath.Join(home, "dev", "abaplint")
	}
	for _, path := range []string{filepath.Join(root, "node_modules", "chai"), filepath.Join(root, "packages", "core", "node_modules", "chai")} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(root, "node_modules", "chai")
}
