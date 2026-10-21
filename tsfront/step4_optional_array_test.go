package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestStep4OptionalNullableArrayFunction(t *testing.T) {
	source := `class Probe {
 static values:string[]=["x"];
 static calls=0;
 static get():string[]|null {Probe.calls++;return Probe.values;}
 static nil():string[]|null {Probe.calls++;return null;}
 static pick(fn:(()=>string[]|null)|undefined):string[] {return fn?.() ?? [];}
 static run():string {const a=Probe.pick(Probe.get);a.push("late");return (a===Probe.values)+":"+Probe.values.join(",")+":"+Probe.pick(undefined).length+":"+Probe.pick(Probe.nil).length+":"+Probe.calls;}
 }`
	for _, trap := range []bool{false, true} {
		t.Run(fmt.Sprintf("trap=%t", trap), func(t *testing.T) {
			p, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: trap})
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(p); len(errs) != 0 {
				t.Fatal(errs)
			}
			conditionalMatchesNode(t, source, p, "Probe")
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
		})
	}
}
