package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConditionalJoinTokenSet(t *testing.T) {
	source := `class Token {getRow():number{return 1;} getCol():number{return 2;}}
class Node {getTokens():Token[]{return [new Token()];}}
export class Probe {static pick(n:Node|undefined):string {
 const s=new Set(n ? n.getTokens().map(t=>t.getRow()+":"+t.getCol()) : []);
 return ""+s.size;
} static run():string{return Probe.pick(new Node())+":"+Probe.pick(undefined);}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	conditionalMatchesNode(t, source, p, "Probe")
	if got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())"); got != "1:0\n" {
		t.Fatal(got)
	}
}

func conditionalMatchesNode(t *testing.T, source string, p *hir.Program, class string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(file, []byte(source+"\nconsole.log("+class+".run());"), 0600); err != nil {
		t.Fatal(err)
	}
	oracle, err := exec.Command("node", file).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v\n%s", err, oracle)
	}
	got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts."+class, "run")+"())")
	if got != string(oracle) {
		t.Fatalf("Go %q, Node %q", got, oracle)
	}
}

func TestConditionalJoinOptionalBoxingIsLazy(t *testing.T) {
	source := `export class Probe {
 static touched:number=0;
 static make():string|undefined {Probe.touched+=1;return "live";}
 static choose(flag:boolean):string|number|undefined {return flag?Probe.make():9;}
 static absent(flag:boolean,x:string|undefined):string|number|undefined {return flag?x:7;}
 static run():string {
  const a=Probe.choose(false);const before=Probe.touched;const b=Probe.choose(true);
  const missing=Probe.absent(true,undefined);const live=Probe.absent(true,"x");
  return before+":"+Probe.touched+":"+a+":"+b+":"+(missing===undefined)+":"+live;
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	conditionalMatchesNode(t, source, p, "Probe")
}

func TestConditionalJoinReferenceAndOptional(t *testing.T) {
	source := `class Base {value():string{return "base";}}
 class Child extends Base {value():string{return "child";}}
 export class Probe {
 static choose(flag:boolean,a:Base,b:Child):Base|undefined {return flag?a:b;}
 static maybe(flag:boolean,a:Base):Base|undefined {return flag?a:undefined;}
 static run():string {
  const a=new Base();const b=new Child();
  return (Probe.choose(true,a,b)===a)+":"+(Probe.choose(false,a,b)===b)+":"+(Probe.maybe(false,a)===undefined);
 }}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	conditionalMatchesNode(t, source, p, "Probe")
}

func TestConditionalJoinRefusesIncompatibleStorage(t *testing.T) {
	source := `export function choose(flag:boolean,a:Set<string>,b:Set<number>):Set<string>|Set<number>{return flag?a:b;}`
	p := integerProbe(t, source)
	_, diags, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Category == "unsupported-conditional-join" && strings.Contains(d.Message, "set<string> and set<number>") {
			return
		}
	}
	t.Fatalf("missing exact-join refusal: %v", diags)
}

func TestConditionalJoinCollectionIdentity(t *testing.T) {
	source := `export class Probe {
 static chooseSet(flag:boolean,a:Set<string>,b:Set<string>|undefined):Set<string>|undefined {return flag?a:b;}
 static chooseMap(flag:boolean,a:Map<string,string>,b:Map<string,string>|undefined):Map<string,string>|undefined {return flag?a:b;}
 static run():string {
  const a=new Set<string>();const b=new Set<string>();
  const m=new Map<string,string>();const n=new Map<string,string>();
  const selectedSet=Probe.chooseSet(false,a,b);const selectedMap=Probe.chooseMap(true,m,n);
  if(selectedSet!==undefined){selectedSet.add("live");}
  if(selectedMap!==undefined){selectedMap.set("key","value");}
  return (selectedSet===b)+":"+(selectedMap===m)+":"+a.size+":"+b.size+":"+m.get("key")+":"+n.size+":"+(Probe.chooseSet(false,a,undefined)===undefined);
 }}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	conditionalMatchesNode(t, source, p, "Probe")
}

func TestConditionalJoinNarrowedUnionClassBase(t *testing.T) {
	source := `class Base {value():string{return "base";}}
 class A extends Base {row():Base{return new Base();}}
 class B extends Base {value():string{return "b";}}
 class C extends Base {value():string{return "c";}}
 export class Probe {
 static choose(t:Base|undefined):Base|undefined {
  if(!(t instanceof A) && !(t instanceof B) && !(t instanceof C)){return undefined;}
  return t instanceof A?t.row():t;
 }
 static run():string {
  const b=new B();const c=new C();
  return (Probe.choose(b)===b)+":"+(Probe.choose(c)===c)+":"+(Probe.choose(undefined)===undefined);
 }}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	conditionalMatchesNode(t, source, p, "Probe")
}

func TestConditionalJoinCheckedDynamicContext(t *testing.T) {
	source := `export class Probe {
 static text(flag:boolean,value:any):string{return flag?value:"fallback";}
 static run():string{return Probe.text(true,"live")+":"+Probe.text(false,9);}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	conditionalMatchesNode(t, source, p, "Probe")
}
