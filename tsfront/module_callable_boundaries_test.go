package tsfront

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestModuleCallableInitializationBoundaries(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := `class State { static effects=0; static bump(): number {State.effects++;return 1;} }
 function base(n: number): number { return n; }
 describe("early",()=>{late(State.bump());});
 const chain = late;
 const late = base;
 const refusedBody = (): number => { const date = new Date(); return date.getTime(); };
 describe("later",()=>{
  it("no argument effect",()=>{if(State.effects!==0)throw new Error("TDZ evaluated arguments");if(late(2)!==2)throw new Error("bad alias");});
  it("failed alias remains unavailable",()=>{chain(2);});
  it("body refusal is deferred",()=>{refusedBody();});
 });`
	dir := t.TempDir()
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": source})
	lowered, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, lowered)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed, Trapped int
		Initialization          []struct{ Message string }
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || code != 1 || got.Passed != 1 || got.Failed != 0 || got.Trapped != 2 || len(got.Initialization) != 2 {
		t.Fatalf("code %d: %s\n%s", code, stdout, stderr)
	}
	for _, f := range got.Initialization {
		if !strings.Contains(f.Message, "was not initialized") {
			t.Fatal(f.Message)
		}
	}
}

func TestRegistrationProofRejectsComputedNames(t *testing.T) {
	runtime, err := os.ReadFile("testdata/abaplint-tests/runtime/mocha.ts")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := `const rows = [{["input"]:"first"}, {["input"]:"second", extra:true}];
 describe("computed",()=>{rows.forEach(row=>{it(row.input,()=>{});});});
 describe("later",()=>{it("independent",()=>{});});`
	writeTestClosure(t, dir, map[string]string{"runtime/mocha.ts": string(runtime), "test/host.ts": source})
	lowered, err := LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := EmitAbaplintTests(dir, lowered)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, _ := runAbaplintTestModule(t, files)
	var got struct {
		Passed, Failed int
		Initialization []struct{ Status string }
	}
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || got.Passed != 1 || got.Failed != 0 || len(got.Initialization) == 0 {
		t.Fatalf("%s\\n%s", stdout, stderr)
	}
	for _, r := range got.Initialization {
		if r.Status != "trapped" {
			t.Fatal(r.Status)
		}
	}
}

func TestModuleCallableMutationRefuses(t *testing.T) {
	for _, source := range []string{
		`function base(n: number): number {return n;} const alias=base; base=(n:number)=>n+1; class Probe {run():number{return alias(1);}}`,
		`function base(n: number): number {return n;} const alias=base; (base as any)=(n:number)=>n+1; class Probe {run():number{return alias(1);}}`,
		`const {name}=()=>1; class Probe {run():number{return name.length;}}`,
	} {
		p := integerProbe(t, source)
		_, diags, err := p.Lower([]string{"probe.ts"})
		if err != nil {
			t.Fatal(err)
		}
		if !hasBlocking(diags) {
			t.Fatal("mutable target lowered as an immutable alias")
		}
	}
}
