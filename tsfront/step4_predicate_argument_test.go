package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestStep4PredicateUnknownArgumentRetainsRuntimeValue(t *testing.T) {
	source := `class A {kind=1;name():string{return "a";}}
class B {name():string{return "b";}}
function isA(value:unknown,kind:number):value is A {return value instanceof A && value.kind===kind;}
function read(value:A|B):string {
 if(isA(value,0)) return "zero";
 return isA(value,1)?"one":"other";
}
function readArray(values:(A|B)[]):string {
 if(isA(values[0],0)) return "zero";
 return isA(values[0],1)?"one":"other";
}
function common(value:A|B):string {
 if(isA(value,0)) return "zero";
 return value.name();
}
function commonArray(values:(A|B)[]):string {
 if(isA(values[0],0)) return "zero";
 return values[0].name();
}
function collect(values:(A|B)[]):A[] {
 const ret=[];
 for(const value of values) {if(value instanceof A) ret.push(value);}
 return ret;
}
class Probe {static run():string{return read(new A())+"|"+read(new B())+"|"+readArray([new A()])+"|"+readArray([new B()])+"|"+collect([new A(),new B()]).length+"|"+common(new A())+"|"+common(new B())+"|"+commonArray([new A()])+"|"+commonArray([new B()]);}}`
	for _, parameterType := range []string{"unknown", "any"} {
		t.Run(parameterType, func(t *testing.T) {
			probe := strings.ReplaceAll(source, "value:unknown", "value:"+parameterType)
			want := conversionOracle(t, probe)
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": probe}, []string{"probe.ts"})
			got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
			if got != want {
				t.Fatalf("Go %q Node %q", got, want)
			}
		})
	}
}
