package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestStep4BoxedMapConstructorNodeOracle(t *testing.T) {
	source := `interface Expr { kind:string; text:string }
interface Parameter { name:string; default?:string }
class Probe {
 static given(imp:Parameter[], body:Expr):string {
  const given = new Map();
  given.set(imp.find((p) => p.default === undefined)?.name ?? imp[0]?.name, body);
  const alias = given;
  const again:Expr = {kind:"expr",text:"updated"};
  alias.set("ARG", again);
  const out:string[]=[];
  for(const [key,value] of given) out.push(String(key)+"="+value.text);
  return out.join(",")+":"+(given.get(undefined)===body)+":"+(given.get("ARG")===again)+":"+given.size;
 }
 static run():string {const body:Expr={kind:"expr",text:"body"};return Probe.given([],body)+"|"+Probe.given([{name:"ARG"}],body);}
}
`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	dump := hir.Dump(p)

	if !strings.Contains(dump, "map<dynamic,dynamic>") {
		t.Fatal("fixture no longer exercises inferred boxed Map", dump)
	}
	want := conversionOracle(t, source)
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStep4BoxedMapNullableKeyNodeOracle(t *testing.T) {
	boxedMapTSOracle(t, `interface Expr {text:string}
 class Probe {
 static run():string {
  const m=new Map<string|null|undefined,Expr|undefined>();const alias=m;
  const body:Expr={text:"body"},next:Expr={text:"next"};
  m.set(undefined,body).set("undefined",next).set("",undefined);
  alias.set(undefined,next);body.text="mutated";
  const out:string[]=[String(m.size),String(m.get(undefined)===next),String(m.has("")),String(m.get("")===undefined),String(m.has("missing")),String(m.get("missing")===undefined)];
  for(const [k,v] of m) out.push(String(k)+"="+(v===undefined?"absent":v.text));
  out.push(String(alias.delete(undefined)),String(m.delete(undefined)));
  m.set(undefined,body);
  for(const [k,v] of alias) out.push(String(k)+"="+(v===undefined?"absent":v.text));
  const copy=new Map(m);copy.set(undefined,next);
  out.push(String(m.get(undefined)===body),String(copy.get(undefined)===next));
  alias.clear();out.push(String(m.size),String(copy.size));return out.join("|");
 }}
 `)
}

func TestStep4BoxedMapAdversarialNodeOracle(t *testing.T) {
	boxedMapTSOracle(t, `class Item {text="same";items:number[]=[1,2];}
 class Probe {
 static run():string {
  const m=new Map<any,string|undefined>();const alias=m;
  const a=new Item(),b=new Item(),xs=[1,2],ys=[1,2];
  const nan=parseFloat("NaN");const negativeZero=-parseFloat("0");
  m.set(nan,"nan1").set(parseFloat("bad"),"nan2");
  m.set(negativeZero,"zero1").set(0,"zero2");
  m.set(1,"number").set("1","string").set(true,"bool").set("true","textbool");
  m.set(null,"null").set(undefined,"undefined").set("undefined","text");
  m.set(a,"a").set(b,"b").set(xs,"xs").set(ys,"ys");
  m.set("stored",undefined);
  const empty={},empty2={},f=()=>1,g=()=>1;
  m.set(empty,"empty").set(empty2,"empty2").set(f,"function").set(g,"function2");
  m.set(BigInt("1"),"bigint");
  const parsed:any=JSON.parse('{"x":[1]}'),other:any=JSON.parse('{"x":[1]}');
  m.set(parsed,"parsed").set(other,"other");
  const parsedArray:any=parsed.x;
  m.set(parsedArray,"parsedArray");
  a.items.push(3);a.text="changed";
  const out:string[]=[String(m.size),String(m.get(nan)),String(m.get(-0)),String(m.get(1)),String(m.get("1")),String(m.get(true)),String(m.get("true")),String(m.get(null)),String(m.get(undefined)),String(m.get(a)),String(m.get(b)),String(m.get(xs)),String(m.get(ys)),String(m.get(empty)),String(m.get(empty2)),String(m.get(f)),String(m.get(g)),String(m.get(BigInt("1"))),String(m.get(parsed)),String(m.get(other)),String(m.get(parsed.x)),String(m.has("stored")),String(m.has("absent")),String(m.get("stored")===m.get("absent"))];
  for(const [k,v] of m) {out.push(String(v));}
  out.push(String(alias.delete(nan)),String(m.has(nan)),String(alias.delete(a)),String(m.has(b)));
  m.set(parseFloat("NaN"),"reinsert");m.set(a,"again");
  for(const [k,v] of m) out.push(String(v));
  return out.join("|");
 }}
 `)
}

func TestStep4BoxedMapLiveUpdatesNodeOracle(t *testing.T) {
	boxedMapTSOracle(t, `class Probe {static run():string {
  const m=new Map<string|null|undefined,number>();const alias=m;
  m.set(undefined,1).set("b",2).set("c",3);
  const out:string[]=[];
  for(const [k,v] of m) {out.push(String(k)+v);if(k===undefined){alias.delete("b");alias.set("c",4);alias.set(undefined,9);}}
  for(const [k,v] of alias){out.push(String(k)+v);m.delete(k);}
  out.push(String(m.size));return out.join("|");
 }}
 `)
}

func TestStep4BoxedMapAppendStillTraps(t *testing.T) {
	for _, mutation := range []string{`m.set(null,3);`, `m.delete(undefined);m.set(undefined,3);`} {
		source := `class Probe {static run():string {const m=new Map<string|null|undefined,number>();m.set(undefined,1).set("b",2);for(const [k,v] of m) {if(k===undefined && v===1){` + mutation + `}}return "done";}}`
		p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
		want := conversionOracle(t, source)
		if want != "done\n" {
			t.Fatal(want)
		}
		got := runGoHIR(t, p, fmt.Sprintf(`defer func(){r,ok:=recover().(trap);if !ok {panic("missing append refusal")};fmt.Println(r.Source)}();%s()`, goEntry("probe.ts.Probe", "run")))
		if !strings.Contains(got, "probe.ts:") || !strings.Contains(got, "Map for-of appended") {
			t.Fatal(got)
		}
	}
}

func TestStep4BoxedMapValueViewNodeOracle(t *testing.T) {
	boxedMapTSOracle(t, `interface A {readonly name:string;readonly args:readonly string[]} interface B {readonly args:readonly string[];readonly name:string}
 class Probe {static choose(a:Map<any,A>,b:Map<any,B>):Map<any,A>{return b;}
 static run():string {const a=new Map<any,A>(),b=new Map<any,B>();const key={id:"key"};const row:B={args:["x"],name:"B"};b.set(key,row);const v=Probe.choose(a,b);const index=new Map<any,string>();index.set(v,"map");index.set(v.get(key),"row");const out:string[]=[String(v===b),String(v.get(key)===row),String(index.get(b)),String(index.get(row))];v.set(undefined,{name:"U",args:[]});b.set(null,{args:["y"],name:"N"});for(const [k,r] of v)out.push(r.name);v.delete(key);v.set(key,{name:"K",args:[]});out.push(b.get(key)!.name);v.clear();out.push(String(b.size));return out.join("|");}}
 `)
}

// Erasing a string-backed Map does not authorize storing non-string keys in
// its original typed storage. The readonly query view remains live.
func TestStep4BoxedMapStringBackedWriteStillRefuses(t *testing.T) {
	source := `class Probe {static view(m:Map<string,number>):Map<string|null|undefined,number>{return m;}static run():number{const m=new Map<string,number>();const view=Probe.view(m);view.set(undefined,1);return m.size;}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if want := conversionOracle(t, source); want != "1\n" {
		t.Fatal(want)
	}
	got := runGoHIR(t, p, fmt.Sprintf(`defer func(){r,ok:=recover().(trap);if !ok {panic("missing storage refusal")};fmt.Println(r.Source)}();%s()`, goEntry("probe.ts.Probe", "run")))
	if !strings.Contains(got, "writes through a boxed string-key Map view are unsupported") {
		t.Fatal(got)
	}
}

func boxedMapTSOracle(t *testing.T, source string) {
	t.Helper()
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	_, err := emitABAP(t, p)
	if err == nil || !strings.Contains(err.Error(), "probe.ts:") || !(strings.Contains(err.Error(), "SameValueZero") || strings.Contains(err.Error(), "map.insertions") || strings.Contains(err.Error(), "map.view")) {
		t.Fatalf("wanted located ABAP collection refusal, got %v", err)
	}
}

func TestStep4BoxedMapOptionalLayoutStillRefuses(t *testing.T) {
	p, _ := lowerProbe(t, sourceProbe(t, `class Probe {static run():number{const m=new Map<string|undefined,number>();return m.size;}}`))
	for _, err := range hir.Verify(p) {
		if strings.Contains(err.Error(), "unsupported collection key optional<string>") {
			return
		}
	}
	t.Fatal("optional-key layout was admitted without a boxed representation")
}
