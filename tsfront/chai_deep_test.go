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

	"github.com/oisee/abapiti/hir/abap"
)

func TestChaiDeepNodeOracle(t *testing.T) {
	// Use upstream's actual Chai, not node:assert or the shim as its own oracle.
	chai := upstreamChaiModule(t)
	if _, err := os.Stat(chai); err != nil {
		t.Skip("pinned upstream Chai unavailable")
	}
	cases := []struct {
		a, b, operation string
		negated         bool
	}{
		{`1`, `1`, "equal", false}, {`1`, `"1"`, "equal", false},
		{`0`, `-0`, "equal", false},
		{`undefined`, `null`, "equal", false}, {`null`, `null`, "equal", false},
		{`{a:1,b:"x"}`, `{b:"x",a:1}`, "equal", false},
		{`{a:1,b:"x"}`, `{a:1}`, "equal", false},
		{`{a:1,b:"x"}`, `{a:1}`, "include", false},
		{`{a:{x:1,y:2}}`, `{a:{x:1}}`, "include", false},
		{`{a:{x:1,y:2}}`, `{a:{y:2,x:1}}`, "include", false},
		{`{a:1,b:2}`, `{a:1,b:9}`, "include", true},
		{`{a:1,b:2}`, `{a:1,b:2}`, "include", true},
		{`{a:1}`, `{}`, "include", false}, {`{a:1}`, `{}`, "include", true},
		{`[{a:1}, {a:2}]`, `{a:2}`, "include", false},
		{`[{a:1}]`, `{a:2}`, "include", true},
		{`[1,2]`, `[2,1]`, "equal", false}, {`[1,2]`, `[1,2]`, "equal", true},
		{`"hello"`, `"ell"`, "include", false},
		{`JSON.parse('{"x":[1,null,{"s":"😀"}]}')`, `JSON.parse('{"x":[1,null,{"s":"😀"}]}')`, "equal", false},
		{`JSON.parse('{}')`, `{x:undefined}`, "include", false},
		{`JSON.parse('{"x":1}')`, `{x:undefined}`, "include", true},
		{`{x:undefined}`, `{x:undefined}`, "equal", false},
		{`{x:undefined}`, `JSON.parse('{}')`, "equal", false},
	}
	var probe strings.Builder
	probe.WriteString(`function decision(a:any,b:any,include:boolean,negated:boolean):boolean {
 try {const e=negated?expect(a).not.deep:expect(a).deep;if(include){e.include(b);}else{e.equal(b);}return true;}catch(error){return false;}
 }
 function reuse():string {
 const a=expect({x:1});const same=a.deep===a;let after=true;
 try{a.equal({x:1});a.not.equal({x:1});}catch(error){after=false;}
 const b=expect({x:1});b.eql({x:1});let eqlOnly=true;
 try{b.equal({x:1});}catch(error){eqlOnly=false;}
 const c=expect(1);let twice=true;
 try{c.not.not.equal(1);}catch(error){twice=false;}
 return same+":"+after+":"+eqlOnly+":"+twice;
 }
 `)
	var oracle strings.Builder
	fmt.Fprintf(&oracle, "const {expect}=require(%q);\n", chai)
	oracle.WriteString(probe.String())
	for _, c := range cases {
		fmt.Fprintf(&oracle, "console.log(decision(%s,%s,%t,%t));\n", c.a, c.b, c.operation == "include", c.negated)
	}
	oracle.WriteString("console.log(reuse());\n")
	out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]))`, oracle.String()).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	wants := strings.Fields(string(out))
	if len(wants) != len(cases)+1 {
		t.Fatal(string(out))
	}
	for i, c := range cases {
		fmt.Fprintf(&probe, "it(%q,()=>{if(decision(%s,%s,%t,%t)!==%s){throw new Error(%q);}});\n", fmt.Sprintf("deep case %d", i), c.a, c.b, c.operation == "include", c.negated, wants[i], fmt.Sprintf("Node oracle case %d", i))
	}
	fmt.Fprintf(&probe, "it(\"persistent assertion flags\",()=>{const got=reuse();if(got!==%q){throw new Error(\"Node persistent flags oracle: \"+got+\" expected \"+%q);}});\n", wants[len(cases)], wants[len(cases)])
	probe.WriteString(`it("fluent aliases",()=>{
 expect({a:1}).to.deep.equal({a:1});expect({a:1}).to.deep.equals({a:1});expect({a:1}).to.eql({a:1});
 expect({a:1}).to.deep.not.include({a:2});expect({a:1}).to.not.deep.include({a:2});
 });`)
	sources := map[string]string{"test/probe.ts": "import {expect} from '../runtime/chai';\n" + probe.String()}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	dir := t.TempDir()
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
	l, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	// ABAP now emits the same intrinsic boundary. Use a fresh lowering because
	// backend passes mutate HIR; the unchanged Go oracle must remain independent.
	abapLowering, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	abapFiles, err := abap.Emit(abapLowering.Prog)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, source := range abapFiles {
		found = found || strings.Contains(source, "METHOD deepEqual.") && strings.Contains(source, "METHOD deepInclude.") && strings.Contains(source, "READ TABLE seen WITH KEY")
	}
	if !found {
		t.Fatal("missing ABAP Chai equality/include implementation")
	}
	files, err := EmitAbaplintTests(dir, l)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var result struct{ Passed, Failed, Trapped, NotRun int }
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if code != 0 || result.Passed != len(cases)+2 || result.Failed != 0 || result.Trapped != 0 || result.NotRun != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
