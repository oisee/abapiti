package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A structural (non-discriminated) union field declared object admits every JS
// object: a parsed array and a class value pass, exactly as the tagged-family
// validator does. Primitives and null stay refused for a required object field.
const structuralUnionObjectFieldSource = `interface Held {readonly name:string; readonly o:object}
interface Counted {readonly name:string; readonly n:number}
type Slot = Held|Counted;
class Box {v=7}
function read(raw:any):string {const rows = raw as readonly Slot[];let out="";for (const s of rows) out+=s.name+";";return out;}
function token(raw:any):object {const s = (raw as readonly Slot[])[0];return (s as Held).o;}
class Probe {
 static run():string {
  const raw = JSON.parse('[{"name":"A","o":[1,2]},{"name":"B","n":3}]');
  const arr = raw[0].o;
  const tok = token(raw);
  const b = new Box();
  const withClass:any[] = [{name:"C",o:b}];
  const ctok = token(withClass);
  return read(raw)+":"+(tok===arr)+":"+arr.length+":"+read(withClass)+":"+(ctok===b)+":"+(ctok as Box).v;
 }
 static bad(o:any):void {const raw = JSON.parse('[{"name":"X","n":1},{"name":"Y"}]');raw[1].o = o;read(raw);}
 static badString():void {Probe.bad("s");}
 static badNumber():void {Probe.bad(1);}
 static badNull():void {Probe.bad(null);}
}`

func TestStructuralUnionObjectFieldNodeOracle(t *testing.T) {
	source := structuralUnionObjectFieldSource
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	var field *hir.StructuralField
	for _, i := range p.Interfaces {
		for _, m := range i.StructuralMembers {
			for k := range m.Fields {
				if m.Fields[k].Name == "o" {
					field = &m.Fields[k]
				}
			}
		}
	}
	if field == nil || field.Type.Kind != hir.ClassRef || field.Type.Name != hir.RootObject {
		t.Fatal("probe must lower Slot as a structural union")
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	for _, method := range []string{"badString", "badNumber", "badNull"} {
		t.Run(method, func(t *testing.T) {
			main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("non-object o must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", method))
			if got := strings.TrimSpace(runStructuralUnionGo(t, p, main)); got != "refused" {
				t.Fatalf("got %q", got)
			}
		})
	}
}

// A plain object in an object field was refused on both backends before the
// object keyword had a domain; Go and ABAP now admit it and still refuse a string.
func TestStructuralUnionObjectFieldPlainNodeOracle(t *testing.T) {
	source := `interface Held {readonly name:string; readonly o:object}
interface Counted {readonly name:string; readonly n:number}
type Slot = Held|Counted;
function read(raw:any):string {const rows = raw as readonly Slot[];let out="";for (const s of rows) out+=s.name+";";return out;}
class Probe {
 static run():string {return read(JSON.parse('[{"name":"A","o":{"a":1}},{"name":"B","n":3}]'));}
 static bad():void {read(JSON.parse('[{"name":"A","o":"s"}]'));}
}`
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "bad", "cx_sy_range_out_of_bounds")
}
