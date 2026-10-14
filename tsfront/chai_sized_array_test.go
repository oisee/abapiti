package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChaiSizedArrayNodeOracle(t *testing.T) {
	chai := upstreamChaiModule(t)
	probe := `it('sized length',()=>{expect(new Array<number>(3).length).equal(3);expect(new Array<any>(3)).length(3);});
it('sized fill',()=>{expect(new Array<number>(2).fill(0)).deep.equal([0,0]);});
it('sized nonzero fill',()=>{expect(new Array<number>(2).fill(1)).deep.equal([1,1]);});
it('holes eql',()=>{expect(new Array<any>(1)).eql([undefined]);});
it('holes reverse eql',()=>{expect([undefined]).eql(new Array<any>(1));});
it('fill identity',()=>{const a=new Array<number>(2);const same=a.fill(0);expect(same).equal(a);expect(a).eql([0,0]);});
it('foreach skip',()=>{let visits=0;new Array<number>(3).forEach(()=>{visits++;});expect(visits).equal(0);});`
	oracle := `const {expect}=require(process.argv[1]);const it=(_,f)=>f();` + probe + `console.log("ok");`
	out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[2]))`, chai, oracle).CombinedOutput()
	if err != nil || string(out) != "ok\n" {
		t.Fatalf("Node: %v %s", err, out)
	}
	sources := map[string]string{"test/probe.ts": "import {expect} from '../runtime/chai';" + probe}
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
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got.Passed != 7 || got.Failed != 0 || got.Trapped != 0 {
		t.Fatalf("code=%d %s %s", code, stdout, stderr)
	}
	if osg := os.Getenv("ABAPITI_OSG_HOME"); osg != "" {
		// Sparse equality and floating-array zero sign remain uncertified.
		// Certified integer zero-fill is covered by the emitter fixture;
		// nonzero fill, length and presence-aware forEach are supported.
		l, err := LowerAbaplintTests(dir)
		if err != nil {
			t.Fatal(err)
		}
		files, err := emitABAPTestsOSG(t, dir, l)
		if err != nil {
			t.Fatal(err)
		}
		out := t.TempDir()
		for name, src := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
		run := exec.Command("node", "../tools/abaplint-tests-osg.mjs", out, t.TempDir(), osg)
		var stdout, stderr strings.Builder
		run.Stdout, run.Stderr = &stdout, &stderr
		err = run.Run()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("OSG: %v %s %s", err, stdout.String(), stderr.String())
		}
		var got struct {
			Passed, Failed, Trapped int
			Results                 []struct{ Title, Status, Message string }
		}
		if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil || got.Passed != 3 || got.Failed != 0 || got.Trapped != 4 {
			t.Fatalf("OSG: %s %s", stdout.String(), stderr.String())
		}
		refusals := map[string]string{"holes eql": "chai.array_items", "holes reverse eql": "chai.array_items", "sized fill": "chai.equal", "fill identity": "chai.equal"}
		for _, row := range got.Results {
			if reason, refused := refusals[row.Title]; refused && (row.Status != "trapped" || !strings.Contains(row.Message, reason)) {
				t.Fatalf("uncertified observation must loudly refuse: %+v", row)
			}
		}
		t.Log("OSG: 3 pass / 0 FAIL / 2 sparse equality traps / 2 zero-sign traps")
	}
}

func TestSizedArrayFillABAPNodeOracle(t *testing.T) {
	source := `class Probe {
static a:number[]=[];static effects='';
static receiver():number[] {Probe.effects+='r';return Probe.a;}
static value():number {Probe.effects+='v';Probe.a.push(7);return 0;}
static run():string {
Probe.a=new Array<number>(2);
const same=Probe.receiver().fill(Probe.value());
return Probe.a.length+':'+Probe.a[0]+':'+Probe.a[1]+':'+Probe.a[2]+':'+(same===Probe.a)+':'+Probe.effects;
}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
}
