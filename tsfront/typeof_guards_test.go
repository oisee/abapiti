package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypeofStructuredGuardsNodeOracle(t *testing.T) {
	source := `class Metadata { R3_NAME?: string; }
export class Probe {
 static calls=0;
 static valueFunctionTag(fn:(x:number)=>number): string {const value:any=fn;return typeof fn+"|"+typeof value;}
 static functionTag(fn:()=>void): string {const value:any=fn;return typeof value+"|"+(typeof value === "function")+"|"+(typeof value !== "object");}
 static recv(x: Metadata): Metadata {Probe.calls++;return x;}
 static guard(x: Metadata): string {
  const tag=typeof Probe.recv(x).R3_NAME;
  if(typeof x.R3_NAME === "string") {return tag+":"+x.R3_NAME;}
  return tag;
 }
 static run(): string {
  const rows: Metadata[]=JSON.parse('[{"R3_NAME":{"nested":""}},{"R3_NAME":"ok"},{"R3_NAME":7},{"R3_NAME":false},{"R3_NAME":null},{}]');
  let out="";
  for(const row of rows) {out=out+Probe.guard(row)+"|";}
  const native=new Metadata();
  out=out+Probe.guard(native)+"|";
  native.R3_NAME="";
  out=out+Probe.guard(native)+"|";
  const dynamic:any=JSON.parse('[null,{},[],"",7,false]');
  for(const value of dynamic) {out=out+typeof value+"|";}
  out=out+Probe.functionTag(()=>{})+"|"+Probe.valueFunctionTag((x:number)=>x)+"|";
  const absent:any=undefined;
  return out+typeof absent+"|"+typeof Probe+"|"+Probe.calls;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("probe.ts.Probe", "run")+"().String())")
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSpace(string(out)) {
		t.Fatalf("Go %q Node %q", got, out)
	}
}
