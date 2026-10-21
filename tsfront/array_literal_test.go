package tsfront

import (
	"strings"
	"testing"

	gohir "github.com/oisee/abapiti/hir/golang"
)

// Dense literals are emitted as one composite literal (len=cap=n) instead of
// a fresh empty array grown by a push per element; the empty literal and any
// literal with a spread keep their original path.
func TestArrayLiteralItemsEmission(t *testing.T) {
	source := `class Probe {static run(x:string):string {
 const a=[x];const b=[x,"y",x];const s=[x];const spread=[x,...s];const e:string[]=[];
 return a.join()+b.length+spread.length+e.length;}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, err := gohir.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	body := files["hir.go"]
	if got := strings.Count(body, "&array[jsString]{Items:"); got != 3 {
		t.Fatalf("folded %d dense literals (a, b, s), want 3:\n%s", got, body)
	}
	if !strings.Contains(body, "&array[jsString]{}") {
		t.Fatalf("empty literal no longer empty:\n%s", body)
	}
	if got := strings.Count(body, ".push("); got != 1 {
		t.Fatalf("spread literal kept %d pushes, want 1:\n%s", got, body)
	}
}

// Element expressions keep their left-to-right evaluation, nested and
// union/optional-element literals fold too, and the built array behaves like
// any other array afterwards.
func TestArrayLiteralItemsNodeOracle(t *testing.T) {
	want := conversionOracle(t, arrayLiteralOracleSource)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": arrayLiteralOracleSource}, []string{"probe.ts"})
	if got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())"); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

const arrayLiteralOracleSource = `class Probe {
 static log:string[]=[];
 static f():string {Probe.log.push("f");return "F";}
 static g():string {Probe.log.push("g");return "G";}
 static arg(values:string[]):string {return values.join("|");}
 static run():string {return Probe.inner(undefined);}
 static inner(x?:string):string {
  const order=[Probe.f(),Probe.g()];
  const nested=[[1,2],[3,4]];const deep=[[[1],[2]],[[3]]];
  const u:(string|number)[]=[x??"u",1];
  const opt:(string|undefined)[]=[x,undefined,"o"];
  const direct=Probe.arg(["a",x??"b","c"]);
  const mutate=[1,2,3];mutate.push(4);mutate.pop();mutate[0]=9;mutate.shift();
  const capProbe=[7];const grown=capProbe.concat([8,9]);
  const out:string[]=[];
  out.push(Probe.log.join(","));out.push(order.join(""));
  out.push(nested.length+":"+String(JSON.stringify(nested))+String(JSON.stringify(deep)));
  out.push(String(JSON.stringify(u)));out.push(String(JSON.stringify(opt)));out.push(direct);
  out.push(String(JSON.stringify(mutate))+":"+mutate.length);out.push(String(JSON.stringify(grown)));
  return out.join(";");
 }
 }`
