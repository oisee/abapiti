package tsfront

import (
	"github.com/oisee/abapiti/hir/abap"
	"testing"
)

func TestPrivateAnyProducerReceiver(t *testing.T) {
	source := `interface Params {getDefaultImporting():string;} class MethodParameters implements Params {getDefaultImporting():string{return "value";}}
 interface Definition {getParameters():Params;} class Method implements Definition {getParameters():Params{return new MethodParameters();}}
 class Probe {private find(present:boolean):any {const method:Definition|undefined=present?new Method():undefined;if(method){return method;}}
 static run():string {const p=new Probe();const yes=p.find(true)?.getParameters().getDefaultImporting();const no=p.find(false)?.getParameters().getDefaultImporting();return yes+":"+(no===undefined);}}
 `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, p, "fmt.Print("+goEntry("probe.ts.Probe", "run")+"())"); got != "value:true" {
		t.Fatal(got)
	}
}
