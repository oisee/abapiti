package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

// A JSON array already has a shared any[] reference adapter. Consuming it
// must not require a new iterator ABI or a snapshot/materialization.
func TestDynamicArrayForOfNodeOracle(t *testing.T) {
	source := `class Probe {
 static effects=0;
 static read(value:any):any { Probe.effects++;return value; }
 static run():string {
  let out="";
  for(const raw of ['{}','{"values":null}','{"values":[]}','{"values":[{"low":"A"},{"low":"B"}]}']) {
   const parsed=JSON.parse(raw);
   for(const value of Probe.read(parsed.values) ?? []) {out+=String(value.low);}
   out+=";";
  }
  const data:any=JSON.parse('["a","b","c"]');
  const storage:any[]=data;
  let seen="";
  for(const item of Probe.read(data)) {
   seen+=String(item);
   if(item==="a") {storage[1]="changed";storage.push("d");}
  }
  const alias:any[]=data;
  let count=0;for(const item of Probe.read(data)) {count++;if(count===1)alias.splice(1);}
  const empty:any=[];for(const item of empty){out+="bad";}
  return out+":"+seen+":"+count+":"+Probe.effects+":"+data.length;
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err = emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestDynamicArrayForOfRefusesOtherPayloads(t *testing.T) {
	source := `class Probe {static object():any {return JSON.parse('{}');} static run(value:any):void {for(const item of value ?? []) {}}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	entry := goEntry("probe.ts.Probe", "run")
	got := runGoHIR(t, p, fmt.Sprintf(`for _,v:=range []*dynamic{box(str("text")),%s(),box(float64(0)),box(false)} {func(){defer func(){if _,ok:=recover().(rangeFault);!ok{panic("missing checked array refusal")}}();%s(v)}()};fmt.Println("refused")`, goEntry("probe.ts.Probe", "object"), entry))
	if got != "refused\n" {
		t.Fatal(got)
	}
}
