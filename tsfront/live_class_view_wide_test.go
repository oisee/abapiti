package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestLiveClassViewWideShapeNodeOracle(t *testing.T) {
	var fields, values []string
	for i := 0; i < 45; i++ {
		fields = append(fields, fmt.Sprintf("f%02d:number", i))
		values = append(values, fmt.Sprintf("f%02d:%d", i, i))
	}
	source := `interface Wide{` + strings.Join(fields, ";") + `;items?:string[]}
 interface Slot {setConfig(conf:any):void;}
 class Holder implements Slot {conf:Wide={` + strings.Join(values, ",") + `};setConfig(conf:Wide):void {this.conf=conf;}}
 class Probe {static run():string {const h=new Holder();const native=h.conf.f07+h.conf.f44;const input=JSON.parse('{"f07":70,"f44":440,"extra":"kept"}');const s:Slot=h;s.setConfig(input);const seen=h.conf.f07+h.conf.f44;h.conf.f01=10;return native+"|"+seen+"|"+input.f01+"|"+(h.conf.f02===undefined)+"|"+(h.conf===input);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, err := emitABAP(t, p)
	if err != nil {
		t.Fatal(err)
	}
	exportLiveViewABAP(t, p, "live-wide", strings.TrimSpace(string(want)), false)
	wide := ""
	for name, text := range files {
		if strings.Contains(text, "* TS: probe.ts") && strings.Contains(text, " Wide\n") {
			wide = strings.TrimSuffix(name, ".clas.abap")
		}
	}
	if wide == "" {
		t.Fatal("wide shape class not emitted")
	}
	for name, text := range files {
		if strings.Contains(text, "NEW "+wide+"( view_source") {
			t.Fatalf("%s: view NEW passes constructor arguments to a wide shape without CONSTRUCTOR", name)
		}
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
