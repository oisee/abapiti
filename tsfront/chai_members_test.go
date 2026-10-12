package tsfront

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestChaiMembersSparseRefusalNodeOracle(t *testing.T) {
	chai := upstreamChaiModule(t)
	out, err := exec.Command("node", "-e", `const {expect}=require(process.argv[1]);const hole=[];hole[2]=1;try {expect(hole).members([undefined,undefined,1]);console.log(true)}catch(e){console.log(false)}`, chai).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if strings.TrimSpace(string(out)) != "false" {
		t.Fatal(string(out))
	}
	sources := map[string]string{"test/probe.ts": `import {expect} from '../runtime/chai';it("sparse members refuses",()=>{const hole:number[]=[];hole[2]=1;expect(hole).members([undefined,undefined,1]);});`}
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
		Passed, Failed, Trapped int
		Results                 []struct{ Status, Message string }
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 1 || got.Failed != 0 || got.Trapped != 1 || !strings.Contains(got.Results[0].Message, "members requires dense arrays") {
		t.Fatalf("code=%d %s %s", code, stdout, stderr)
	}
}
