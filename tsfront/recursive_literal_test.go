package tsfront

import (
	"fmt"
	"testing"

	"github.com/oisee/abapiti/hir"
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

func TestBoxedCanonicalArrayLiteralStorage(t *testing.T) {
	source := `class Config { names:string[]=[]; }
 function read(c:Config):string {return c.names.join(",");}
 class Probe {static run():string {return read({names:["one","two"]});}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	arrays := 0
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			walkNumberStmt(m.Body, func(*hir.Stmt) {}, func(e *hir.Expr) {
				if e.Kind == hir.New && e.Type.Kind == hir.Array {
					arrays++
					if !e.Type.Equal(hir.T(hir.Array, hir.T(hir.String))) {
						t.Errorf("boxed canonical literal changed storage: %s", e.Type)
					}
				}
			})
		}
	}
	if arrays == 0 {
		t.Fatal("no literal array storage found")
	}
	gogenCheck(t, p, "probe.ts", source, "one,two")
}
