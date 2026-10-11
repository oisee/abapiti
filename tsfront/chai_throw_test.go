package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestChaiThrowNodeOracle(t *testing.T) {
	chai := "/home/alice/dev/abaplint/node_modules/chai"
	if _, err := os.Stat(chai); err != nil {
		t.Skip("pinned upstream Chai unavailable")
	}
	probe := `class State {static calls=0;}
 function decision(throws:boolean,text:string,expected:string,negated:boolean):boolean {
  try {const assertion=negated?expect(()=>{State.calls++;if(throws){throw new Error(text);}}).not:expect(()=>{State.calls++;if(throws){throw new Error(text);}});assertion.to.throw(expected);return true;}catch(error){return false;}
 }
 function plain():boolean {try {expect(()=>{throw new Error("boom");}).to.throw();return true;}catch(error){return false;}}
 function stringThrows():string {
 let a=true;let b=true;let c=true;
 try{expect(()=>{throw "downport, unable to downport table expression in ELSEIF";}).to.throw("downport, unable to downport table expression in ELSEIF");}catch(error){a=false;}
 try{expect(()=>{throw "prefix boom suffix";}).to.throw("boom");}catch(error){b=false;}
 try{expect(()=>{throw "other";}).not.to.throw("boom");}catch(error){c=false;}
 return a+":"+b+":"+c;
 }
 function decisions():string {return stringThrows()+":"+decision(false,"","",false)+":"+decision(false,"","",true)+":"+
 decision(true,"boom","",false)+":"+decision(true,"boom","",true)+":"+decision(true,"prefix boom suffix","boom",false)+":"+
 decision(true,"other","boom",false)+":"+decision(true,"other","boom",true)+":"+decision(true,"boom","boom",true)+":"+plain()+":"+State.calls;}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]))`, fmt.Sprintf("const {expect}=require(%q);\n", chai)+probe+"\nconsole.log(decisions());").CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	for _, normalize := range []bool{false, true} {
		t.Run(fmt.Sprint(normalize), func(t *testing.T) {
			sources := map[string]string{"test/probe.ts": "import {expect} from '../runtime/chai';\n" + probe + fmt.Sprintf(`
 it("throw oracle",()=>{const got=decisions();if(got!==%q){throw new Error("Node oracle: "+got);}});
 it("genuine missing throw",()=>{expect(()=>{}).to.throw();});
 it("unsupported callback remains refused",()=>{expect(()=>new Date()).to.throw();});`, strings.TrimSpace(string(want)))}
			for _, name := range []string{"mocha", "chai"} {
				raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
				if err != nil {
					t.Fatal(err)
				}
				sources["runtime/"+name+".ts"] = string(raw)
			}
			if normalize {
				sources["test/probe.ts"] += `it("existing normalized thunk mode",async()=>{});`
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
			if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 1 || got.Passed != map[bool]int{false: 1, true: 2}[normalize] || got.Failed != 1 || got.Trapped != 1 || got.NotRun != 0 || len(got.Initialization) != 0 {
				t.Fatalf("code=%d %s\n%s", code, stdout, stderr)
			}
			if got.Results[1].Status != "failed" || got.Results[2].Status != "trapped" {
				t.Fatal(got.Results)
			}
		})
	}
}
