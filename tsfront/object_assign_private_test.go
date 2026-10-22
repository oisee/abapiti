package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestObjectAssignPrivateSlotsNodeAndABAP(t *testing.T) {
	source := `class Box {
 static seed=1;
 #n=Box.seed;
 publicValue=0;
 get():number {return this.#n;}
 }
 class HashProperty { "#n"=3; get():number {return this["#n"];} }
 class Probe {
 static target=new Box();
 static refused():void {Object.assign(Probe.target,{"#n":7});}
 static run():string {
  Box.seed=1; const a=new Box();
  Box.seed=2; const b=new Box(); b.publicValue=9;
  Object.assign(a,b);
  const hash=new HashProperty(); Object.assign(hash,{"#n":7});
  return a.get()+":"+b.get()+":"+a.publicValue+":"+hash.get()+":"+Probe.target.get();
 }
 }`
	want := regexpOptionalCapturesNode(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf(`fmt.Println(%s());func(){defer func(){if _,ok:=recover().(trap);!ok{panic("unrepresentable ordinary property must refuse")}}();%s();panic("missing refusal")}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "refused"), goEntry("probe.ts.Probe", "run")))
	if got != want+want {
		t.Fatalf("Go %q Node %q", got, want+want)
	}
	// The nominal Box has no storage for the ordinary string key "#n". Refuse
	// that write, then rerun the oracle to prove private state survived it.
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "refused", "view.refusal", "run")
	// Node can create the ordinary property and still reads the private slot.
	nodeSource := strings.Replace(source, "Object.assign(a,b);", "Object.assign(a,b); Probe.refused();", 1)
	if nodeWant := regexpOptionalCapturesNode(t, nodeSource); nodeWant != want {
		t.Fatalf("private slot changed in Node: %q != %q", nodeWant, want)
	}
}

func TestObjectAssignPrivateSlotPropertyCollisionRefused(t *testing.T) {
	source := `class Box {#n=1; "#n"=2; get():number {return this.#n;}} class Probe {static run():number {const b=new Box();Object.assign(b,{"#n":7});return b.get();}}`
	ds := lowerFamilyDiagnostics(t, source)
	for _, d := range ds {
		if d.Category == "unsupported-member" && strings.Contains(d.Message, "distinct storage") {
			return
		}
	}
	t.Fatalf("missing private-slot collision diagnostic: %v", ds)
}
