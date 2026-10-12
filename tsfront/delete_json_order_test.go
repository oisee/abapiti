package tsfront

import "testing"

// Exercise the real delete lowering together with native JSON own-key storage,
// not just a manually edited bag. Base-typed writes must update the derived
// object's shared order, clear its absence marker, and evaluate receivers once.
func TestDeleteReaddJSONOrderNodeOracle(t *testing.T) {
	source := `class Base {first="first";value?:string="value";last="last";}
class Derived extends Base {extra="extra";}
class Probe {
 static calls=0;
 static recv(x:Base):Base {Probe.calls++;return x;}
 static run():string {
  const x:Base=new Derived();
  let out=JSON.stringify(x)+"/"+Object.keys(x).join(",");
  delete Probe.recv(x).value;
  out+="/"+JSON.stringify(x)+"/"+Object.keys(x).join(",")+"/"+(x.value===undefined);
  Probe.recv(x).value="again";
  out+="/"+JSON.stringify(x)+"/"+Object.keys(x).join(",");
  delete x.value;
  x.value=undefined;
  out+="/"+JSON.stringify(x)+"/"+Object.keys(x).join(",");
  x.value="last-again";
  return out+"/"+JSON.stringify(x)+"/"+Object.keys(x).join(",")+"/"+Probe.calls;
 }
}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"().String())")
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
