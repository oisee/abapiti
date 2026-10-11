package tsfront

import (
	"fmt"
	"testing"
)

func TestGoOptionalDynamicReferenceProjection(t *testing.T) {
	source := `class Probe {static take(value: Map<string,string>|undefined):string {return value===undefined?"absent":value.get("x") || "missing";} static run():string {let value=undefined;const before=Probe.take(value);value=new Map<string,string>();value.set("x","live");return before+":"+Probe.take(value);}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "absent:live\n" {
		t.Fatal(got)
	}
}
func TestGoOptionalInterfaceIdentity(t *testing.T) {
	source := `interface ValueNode {value():string;} class A implements ValueNode {value():string {return "a";}} class B implements ValueNode {value():string {return "b";}}
 class Probe {static find(a:A,n:(A|B)|undefined):boolean{return a===n;} static run():string {const a=new A();return Probe.find(a,a)+":"+Probe.find(a,new A())+":"+Probe.find(a,undefined);}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "true:false:false\n" {
		t.Fatal(got)
	}
}
