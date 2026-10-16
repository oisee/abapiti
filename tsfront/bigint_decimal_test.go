package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestBigIntDecimalNodeOracle(t *testing.T) {
	source := `const MIN=-(1n<<63n),MAX=(1n<<63n)-1n;
function literal(s:string):bigint|null {return /^-?\d+$/.test(s)?BigInt(s):null;}
class Probe {static run():string {
 const values:string[]=['','  +000  ','\ufeff-000\u00a0','000','-000','123','-123','9223372036854775807','9223372036854775808','-9223372036854775808','-9223372036854775809','1234567890123456789012345678901234567890'];
 let out='';for(const s of values){const v=literal(s);if(v!==null)out+=v+':'+(-v)+':'+(v>=MIN&&v<=MAX)+':'+(v===BigInt(s))+':'+typeof v+';';}
 out+=String(BigInt(' +00123 '))+':'+String(BigInt('\ufeff-000\u00a0'))+':'+String(BigInt(''))+':'+(!0n)+':'+(!1n);return out;
}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
