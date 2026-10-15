package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// familyCommonSwitchTypes is a recursive family behind intersected metadata:
// line is common, type is common with a narrower payload in num and an
// optional slot in flag, x differs per member and needs narrowing. TypedExpr
// and ValueExpr intersect type again, which distributes into never parts.
const familyCommonSwitchTypes = `type Meta = {line:number};
type Ty = {k:'i'} | {k:'ref'; name:string} | {k:'table'; row:Ty};
type Expr = Meta & ({e:'field'; base:Expr; name:string; type:Ty; x:Expr} | {e:'num'; value:number; type:Extract<Ty,{k:'i'}>; x:number} | {e:'flag'; value:boolean; type?:Ty; x:string} | {e:'row'; base:Expr; index:Expr; type:Ty; x:boolean});
type TypedExpr = Expr & {type:Ty};
type ValueExpr = (TypedExpr | {e:'void'; line:number; type:Extract<Ty,{k:'i'}>}) & {arith?:boolean};
`

const familyCommonSwitchProbe = familyCommonSwitchTypes + `class Probe {
 static ty(t:Ty):string {switch (t.k) {case 'i': return 'i'; case 'ref': return 'ref:'+t.name; default: return 'table<'+Probe.ty(t.row)+'>';}}
 static common(p:Expr):string {const t=p.type; return p.line+':'+(t===undefined?'-':Probe.ty(t))+':'+p.e;}
 static show(p:Expr):string {
  switch (p.e) {
   case 'num': return 'n'+(p.x+p.value)+'/'+p.type.k;
   case 'flag': return 'f'+p.x.length+(p.value?'T':'F');
   case 'field': case 'row': {
    const head = p.e==='field' ? 'd'+p.name : 'r'+Probe.show(p.index);
    return head+'('+Probe.show(p.base)+')'+p.type.k;
   }
  }
  return '?';
 }
 static others(p:Expr):string {
  switch (p.e) {
   case 'num': return 'num';
   default: return 'other:'+p.line+':'+(p.type===undefined?'-':p.type.k);
  }
 }
 static nested(p:Expr):string {
  if (p.e !== 'field' && p.e !== 'row') return 'leaf';
  switch (p.type.k) {
   case 'ref': return 'ref='+p.type.name;
   case 'table': return 'table='+p.type.row.k+'/'+Probe.nested(p.base);
   default: return 'i/'+Probe.nested(p.base);
  }
 }
 static valued(p:ValueExpr):string {return Probe.ty(p.type)+p.line+(p.arith?'A':'');}
 static early(p:Expr):number {
  if (p.e !== 'num') return -1;
  return p.x*10+p.value;
 }
 static run():string {
  const n:Expr={e:'num',line:1,value:2,type:{k:'i'},x:3};
  const f:Expr={e:'flag',line:2,value:true,x:'abc'};
  const g:Expr={e:'flag',line:3,value:false,type:{k:'ref',name:'R'},x:''};
  const d:Expr={e:'field',line:4,base:n,name:'F',type:{k:'table',row:{k:'ref',name:'Q'}},x:f};
  const r:Expr={e:'row',line:5,base:d,index:n,type:{k:'ref',name:'S'},x:true};
  const all:Expr[]=[n,f,g,d,r];
  const out:string[]=[];
  for (const p of all) out.push(Probe.common(p)+'|'+Probe.show(p)+'|'+Probe.others(p)+'|'+Probe.nested(p)+'|'+Probe.early(p));
  const vs:ValueExpr[]=[{e:'num',line:6,value:2,type:{k:'i'},x:3},{e:'void',line:7,type:{k:'i'},arith:true},{e:'flag',line:8,value:true,x:'abc',type:{k:'ref',name:'V'}}];
  for (const v of vs) out.push(Probe.valued(v));
  return out.join(';');
 }
}`

func TestFamilyCommonFieldAndSwitchNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, familyCommonSwitchProbe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": familyCommonSwitchProbe}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "union.") {
		t.Fatalf("ABAP did not explicitly refuse the recursive family ABI: %v", err)
	}
}

// A clause that reassigns its subject loses the checker's narrowing, so the
// member-specific field keeps the old refusal at its location.
func TestFamilySwitchReassignedRefusal(t *testing.T) {
	source := familyCommonSwitchTypes + `class Probe {
 static bad(p:Expr, other:Expr):string {
  let q = p;
  switch (q.e) {
   case 'num': q = other; return typeof q.x;
   default: return '';
  }
 }
}`
	for _, d := range lowerFamilyDiagnostics(t, source) {
		if !strings.HasPrefix(d.Category, "note-") && strings.Contains(d.Message, "recursive union field x requires narrowing") && strings.HasPrefix(d.Loc, "probe.ts:10:") {
			return
		}
	}
	t.Fatalf("reassigned subject was narrowed: %v", lowerFamilyDiagnostics(t, source))
}
