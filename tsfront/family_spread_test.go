package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// Distributive spread over a recursive family: {...x, line} and
// {...x, ...meta} keep the source's own keys in insertion order, then the
// overrides (an existing key keeps its position), absent and present
// undefined apart; the result is admitted as the member with its tag.
const familySpreadSource = `type Meta = {line?: number; label?: string};
type Ty = {k:'i'} | {k:'table'; row:Ty};
type Expr = Meta & ({e:'num'; value:number; type:Ty} | {e:'add'; l:Expr; r:Expr; type:Ty} | {e:'flag'; value:boolean; type?:Ty});
class Probe {
 static withLine(x:Expr, line:number):Expr { return {...x, line}; }
 static tagged(x:Expr, meta:Meta):Expr { return {...x, ...meta}; }
 static show(x:Expr):string { return JSON.stringify(x)+'/'+Object.keys(x).join(','); }
 static run():string {
  const n:Expr = {e:'num', value:1, type:{k:'i'}};
  const a:Expr = {label:'L', e:'add', l:n, r:n, type:{k:'table', row:{k:'i'}}};
  const f:Expr = {e:'flag', value:true};
  const g:Expr = {e:'flag', value:false, type:undefined};
  return [Probe.show(Probe.withLine(n, 3)), Probe.show(Probe.withLine(a, 4)), Probe.show(Probe.tagged(f, {label:'x', line:undefined})), Probe.show(Probe.tagged(g, {})), Probe.show(Probe.tagged(a, {line:9}))].join(' ; ');
 }
}
`

func TestFamilySpreadNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, familySpreadSource).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": familySpreadSource}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "union.") {
		t.Fatalf("ABAP did not explicitly refuse the recursive family ABI: %v", err)
	}
}

// A spread whose result fits no member of the declared target is refused at
// its location.
func TestFamilySpreadNoMemberRefusal(t *testing.T) {
	source := `type Tree = {k:'leaf'; v:number} | {k:'node'; kids:Tree[]};
type Other = {k:'leaf'; v:number} | {k:'odd'; w:string};
class Probe {static bad(o:Other):Tree {return {...o} as any as Tree;} static worse(o:Other):Tree {
 // @ts-ignore odd fits no Tree member
 return {...o};}}`
	for _, d := range lowerFamilyDiagnostics(t, source) {
		if !strings.HasPrefix(d.Category, "note-") && strings.Contains(d.Message, "fits no member") && strings.HasPrefix(d.Loc, "probe.ts:5:") {
			return
		}
	}
	t.Fatalf("unfit spread member was not refused: %v", lowerFamilyDiagnostics(t, source))
}
