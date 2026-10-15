package tsfront

import (
	"fmt"
	"testing"
)

func TestRecursiveTupleLiteralNodeOracle(t *testing.T) {
	// The overload supplies the recursive checker context while the implementation
	// uses the existing array ABI. The finite literal must still evaluate every
	// nested element in source order when that context cannot be laid out.
	source := `type ASTTuple = readonly [string, Record<string,string>, readonly ASTTuple[]];
 function visit(tree:ASTTuple):string;
 function visit(tree:readonly any[]):string {
  return tree[0]+":"+tree.length+":"+JSON.stringify(tree[1]);
 }
 class Probe {
 static trace="";
 static mark(value:string):string {Probe.trace+=value+";";return value;}
 static run():string {Probe.trace="";return visit([Probe.mark("Select"),{text:Probe.mark("root")},[
  [Probe.mark("Fields"),{},[[Probe.mark("Field"),{},[[Probe.mark("Identifier"),{text:Probe.mark("col")},[] as any[]]]]]]
 ]])+":"+Probe.trace;}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if t.Failed() {
		return
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
