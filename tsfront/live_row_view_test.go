package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestLiveConfigRowViewsNodeOracle(t *testing.T) {
	for _, declaration := range []string{
		`type Row={name:string,replace:string,from?:string};`,
		`interface Row {name:string;replace:string;from?:string;}`,
	} {
		source := declaration + `class Base {severity=1;}
 class Conf extends Base {rows:Row[]=[];}
 interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void{this.conf=conf;}}
 class Probe {
 static plain():string {const original={name:"a",replace:"b",extra:"kept"};const input={rows:[original]};const rule=new Rule();const slot:Slot=rule;slot.setConfig(input);const first=rule.conf.rows[0];original.name="A";const seen=first.name;first.replace="B";first.from="new";const raw:any=first;const originalRef:any=original;return (first===originalRef)+":"+seen+":"+original.replace+":"+raw.extra+":"+raw.from;}
 static run():string {
 const input=JSON.parse('{"rows":[{"name":"a","replace":"b","extra":7},{"name":"c","replace":"d","from":"old"}],"severity":2}');
 const rule=new Rule();const slot:Slot=rule;slot.setConfig(input);
 const rows=rule.conf.rows;const first=rows[0];const again=rule.conf.rows[0];
 const absent=first.from===undefined;const keys=Object.keys(first).join(",");
 const raw=input.rows[0];raw["name"]="A";const seen=first.name;first.replace="B";first.from="new";
 input.rows[1]=JSON.parse('{"name":"e","replace":"f"}');const second=rows[1];
 rows.push(first);const same=first===again;const caller:any=first;
 const answer=(rows===input.rows)+":"+same+":"+(first===input.rows[0])+":"+absent+":"+keys+":"+seen+":"+input.rows[0].replace+":"+input.rows[0].from+":"+caller.extra+":"+second.name+":"+(second.from===undefined)+":"+(rows[2]===first)+":"+rows.length+":"+rule.conf.severity;
 first.from=undefined;const explicit=Object.keys(first).join(",");
 const partial=JSON.parse('{}');slot.setConfig(partial);return answer+":"+(rule.conf.rows===undefined)+":"+Object.keys(partial).length+":"+explicit+":"+(first.from===undefined)+"|"+Probe.plain();
 }}`
		want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(want))
		}
		p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
		if _, err := abap.Emit(p); err != nil {
			t.Fatal(err)
		}
		exportLiveViewABAP(t, p, "config-rows", strings.TrimSpace(string(want)), true)
		if got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())"); got != string(want) {
			t.Fatalf("Go %q Node %q", got, want)
		}
	}
}

func TestErasedLiveStringArrayPresence(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `class Probe {static view(value:any):string[] {const names:string[]=value;return names;}}`}, []string{"probe.ts"})
	// Activate the actual Chai runtime alongside an otherwise primitive-only
	// view program, so the existing live length guard is exercised directly.
	p.Classes = append(p.Classes, &hir.Class{Name: "ChaiProbe", Methods: []*hir.Method{{Name: "box", Static: true, Result: hir.T(hir.Dynamic), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "chai.box", Type: hir.T(hir.Dynamic), X: hir.L(hir.T(hir.I32), int32(1))}})}}})
	main := fmt.Sprintf(`source:=parseJSON(str("[\"a\",\"b\"]"));typed:=%s(source);raw:=source.Value.(*array[any]);
 if !dynEqual(box(typed),source) || typed.chaiLength()!=2 {panic("identity/length")};raw.delete(0);
 if typed.has(0) || typed.get(0).Has {panic("hole became present")};raw.livePut(0,nil);
 if !typed.has(0) {panic("undefined became missing")};
 defer func(){if _,ok:=recover().(trap);!ok {panic("undefined element must refuse")};fmt.Println("refused")}();typed.get(0)`, goEntry("probe.ts.Probe", "view"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func TestErasedLiveStringArrayNodeOracle(t *testing.T) {
	source := `class Probe {
 static view(value:any):string[] {const names:string[]=value;return names;}
 static run():string {const input=JSON.parse('["a","b"]');const typed=Probe.view(input);const same=typed===input;input[0]="A";const seen=typed[0];typed.push("c");typed[1]="B";const other=Probe.view(input);other.splice(1,1,"x");return same+":"+seen+":"+typed.join(",")+":"+input[1]+":"+input.length;}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	exportLiveViewABAP(t, p, "erased-strings", strings.TrimSpace(string(want)), true)
	if got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())"); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestErasedLiveStringArrayRefusals(t *testing.T) {
	for _, body := range []string{
		`const value=JSON.parse('[1]');const typed:string[]=value;return typed[0];`,
		`const value=JSON.parse('["a"]');const typed:string[]=value;value[0]=false;return typed[0];`,
		`const value=JSON.parse('["a"]');const typed:string[]=value;value[0]=undefined;return typed[0];`,
	} {
		source := `class Probe {static run():string {` + body + `}}`
		p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
		main := fmt.Sprintf(`defer func(){v:=recover();tr,ok:=v.(trap);message:="live string array requires a string element";if !ok || len(tr.Source)<len(message) || tr.Source[len(tr.Source)-len(message):]!=message {panic(fmt.Sprintf("unexpected refusal: %%v",v))};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "run"))
		if got := runGoHIR(t, p, main); got != "refused\n" {
			t.Fatal(got)
		}
	}
}
