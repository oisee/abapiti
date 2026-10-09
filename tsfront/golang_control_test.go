package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoRestSuperFinallyNodeOracle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	source := `class Base { value() { return 7; } }
 class Child extends Base { other() { return super.value() + 2; } }
 class Probe {
 static rest(...xs: number[]) { let n=0; for(const x of xs){n+=x;} return n; }
 static run() { let n=0; let i=0; while(i<5) { i++; try { if(i===2){continue;} if(i===4){break;} n+=i; } catch(e) {n+=100;} } try {n+=1;} finally {n+=10;} return n+new Child().other()+Probe.rest(1,2,3); }
 }
`
	node := exec.Command("node", "-e", source+`process.stdout.write(String(Probe.run())+"\n")`)
	want, err := node.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
