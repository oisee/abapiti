package tsfront

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestChaiLiveOrderedRecordNodeOracle(t *testing.T) {
	chai := upstreamChaiModule(t)
	oracle := `const {expect}=require(process.argv[1]);const record={};const same=record;record.x=undefined;expect(record).deep.equal({x:undefined});record.x={value:1};const nested=record.x;nested.value=2;expect(same).deep.equal({x:{value:2}});expect(same).equal(record);expect({}).not.deep.equal({x:undefined});console.log("pass");`
	out, err := exec.Command("node", "-e", oracle, chai).CombinedOutput()
	if err != nil || string(out) != "pass\n" {
		t.Fatalf("Node: %v %s", err, out)
	}
	sources := map[string]string{"test/probe.ts": `import {expect} from '../runtime/chai';it("live record enumerable keys",()=>{
 const record:{[key:string]:any}={};const same=record;record.x=undefined;expect(record).deep.equal({x:undefined});record.x={value:1};const nested=record.x;nested.value=2;expect(same).deep.equal({x:{value:2}});expect(same).equal(record);expect({}).not.deep.equal({x:undefined});
 });`}
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
	var got struct{ Passed, Failed, Trapped int }
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 1 || got.Failed != 0 || got.Trapped != 0 {
		t.Fatalf("code=%d %s %s", code, stdout, stderr)
	}
}
