package tsfront

import (
	"fmt"
	"strings"
	"testing"

	gohir "github.com/oisee/abapiti/hir/golang"
)

// Object literals start with a shared, static own-key names table; the probes
// pin Node's order through every later write, delete and re-add.
func literalOwnKeysNodeOracle(t *testing.T, source string) string {
	t.Helper()
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, err := gohir.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	for _, src := range files {
		all += src
	}
	if !strings.Contains(all, "ownKeyState{names:") {
		t.Fatal("object literal did not get a static own-key table")
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	return all
}

func TestLiteralOwnKeysOrderNodeOracle(t *testing.T) {
	literalOwnKeysNodeOracle(t, `interface Rec {node:number; previous?:Rec; z?:string; a?:string}
class Probe {
 static show(r:Rec):string {
  const f:string[]=[];for(const k in r){f.push(k);}
  const s={...r};
  return Object.keys(r).join(",")+"/"+f.join(",")+"/"+JSON.stringify(r)+"/"+Object.keys(s).join(",");
 }
 static run():string {
  const out:string[]=[];
  const a:Rec={node:1, previous:undefined};
  const b:Rec={node:2, previous:a};
  out.push(Probe.show(a), Probe.show(b));
  const c:Rec={z:"z", node:3};c.a="a";out.push(Probe.show(c));
  const d:Rec={node:4, z:"z", a:"a"};delete d.z;out.push(Probe.show(d));d.z="again";out.push(Probe.show(d));
  const e:Rec={node:5, z:"z", a:"a"};e.node=6;e.z="y";out.push(Probe.show(e));
  const f:Rec={a:"a", node:7};f.previous=e;out.push(Probe.show(f));
  return out.join("\n");
 }
}`)
}

func TestLiteralOwnKeysSharedShapeNodeOracle(t *testing.T) {
	all := literalOwnKeysNodeOracle(t, `interface Rec {node:number; previous?:Rec; extra?:number}
class Probe {
 static make(n:number, p?:Rec):Rec {return {node:n, previous:p};}
 static run():string {
  const out:string[]=[];
  const a=Probe.make(1);const b=Probe.make(2, a);const c:Rec={node:3, previous:b};
  a.extra=1;delete b.previous;
  out.push(Object.keys(a).join(","), Object.keys(b).join(","), Object.keys(c).join(","), JSON.stringify(c));
  b.previous=a;b.node=9;out.push(Object.keys(b).join(","), Object.keys(Probe.make(4)).join(","));
  return out.join("\n");
 }
}`)
	if n := strings.Count(all, "var z_ownkeys_lite"); n != 1 {
		t.Fatalf("names table for one key sequence emitted %d times", n)
	}
}

func TestLiteralOwnKeysIntegerAndNestedNodeOracle(t *testing.T) {
	literalOwnKeysNodeOracle(t, `interface Inner {b:number; a:number}
interface Rec {"1"?:number; "0"?:number; x?:number; "10"?:number; "01"?:number; inner?:Inner}
class Probe {
 static run():string {
  const out:string[]=[];
  const r:Rec={x:1, "1":2, "01":3, "0":4, inner:{b:1, a:2}};
  out.push(Object.keys(r).join(","), JSON.stringify(r));
  r["10"]=5;out.push(Object.keys(r).join(","));
  delete r["1"];r["1"]=6;out.push(Object.keys(r).join(","), JSON.stringify(r));
  const k:string[]=[];for(const n in r.inner!){k.push(n);}out.push(k.join(","));
  return out.join("\n");
 }
}`)
}

func TestLiteralOwnKeysOptionalNodeOracle(t *testing.T) {
	literalOwnKeysNodeOracle(t, `interface Rec {matched:number; unmatched?:number; error?:boolean; errorDescription?:string}
class Probe {
 static make(m:number, fail:boolean):Rec {
  if (fail) {return {matched:m, unmatched:0, error:true, errorDescription:"bad"};}
  return {matched:m, unmatched:undefined};
 }
 static run():string {
  const a=Probe.make(1,false);const b=Probe.make(2,true);
  a.errorDescription="late";
  return Object.keys(a).join(",")+"|"+JSON.stringify(a)+"|"+Object.keys(b).join(",")+"|"+JSON.stringify(b);
 }
}`)
}

// Creating a literal and enumerating its keys keeps the names table: no
// ordered map is materialised.
func TestLiteralOwnKeysNoOrderedMap(t *testing.T) {
	source := `interface Rec {node:number; previous?:Rec}
class Probe {
 static make(n:number, p?:Rec):Rec {return {node:n, previous:p};}
 static keys(r:Rec):string {return Object.keys(r).join(",")+JSON.stringify(r);}
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	mk, keys := goEntry("probe.ts.Probe", "make"), goEntry("probe.ts.Probe", "keys")
	main := fmt.Sprintf(`r:=%s(1,nil);fmt.Println(%s(r), r.ownKeys.m==nil)`, mk, keys)
	got := runGoHIR(t, p, main)
	if want := "node,previous{\"node\":1} true\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
