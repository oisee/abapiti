package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// SunkLocals and Tops key their maps by the identity of body arrays.
func TestGogenA26ArrayMapKeyNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const a=[1], b=[1], alias=a; const m=new Map<number[],string>();
 m.set(a,"first"); m.set(b,"second"); a.push(2); m.set(alias,"updated");
 const s=new Set<number[]>(); s.add(a); s.add(b); s.add(alias);
 const before=m.size+":"+m.get(alias)+":"+m.get(b)+":"+m.has([1])+":"+s.size;
 m.delete(a); s.delete(alias);
 return before+":"+m.has(alias)+":"+s.has(a)+":"+m.get(b);
 }}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	exportLiveViewABAP(t, p, "a26-array-keys", strings.TrimSpace(want), false)
}

// sinkLocals truncates the retained path in place rather than replacing it.
func TestGogenA26ArrayLengthNodeOracle(t *testing.T) {
	source := `class Probe {
 static trace="";
 static recv(a:number[]):number[] {Probe.trace+="L";return a;}
 static size(n:number):number {Probe.trace+="R";return n;}
 static run():string {const a=[1,2,3], alias=a;const n=(Probe.recv(a).length=Probe.size(2));
 const first=alias.join('/')+":"+n+":"+Probe.trace;
 a.length=0; a.push(9); a.length=1;
 return first+":"+alias.join('/')+":"+(a===alias);}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	exportLiveViewABAP(t, p, "a26-length", strings.TrimSpace(want), false)
}

// emitGo uses Boolean(layers); resolve the library global, preserving truthiness.
func TestGogenA26BooleanNodeOracle(t *testing.T) {
	source := `class Probe {static trace="";
 static effect(n:number):number {Probe.trace+=String(n);return n;}
 static value(x:unknown):string {return String(Boolean(x));}
 static run():string {const a=Boolean(Probe.effect(0));
 return [String(Boolean()),Probe.value(undefined),Probe.value(null),Probe.value(false),Probe.value(0),Probe.value(-0),Probe.value(NaN),Probe.value(""),Probe.value("0"),Probe.value([]),Probe.value({}),Probe.value(1),String(a),Probe.trace].join('/');}}
 `
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	exportLiveViewABAP(t, p, "a26-boolean", strings.TrimSpace(want), false)
}

func TestGogenA26ArrayLengthRefusal(t *testing.T) {
	source := `class Probe {static xs=[1,2];static bad(n:number):void {Probe.xs.length=n;} static current():string {return Probe.xs.join('/');}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	for _, length := range []string{"3", "-1", "0.5"} {
		t.Run(length, func(t *testing.T) {
			main := fmt.Sprintf(`defer func(){r:=recover();if _,ok:=r.(trap);!ok {panic(fmt.Sprint("expected refusal: ",r))};fmt.Println(r);fmt.Println(%s())}();%s(%s);panic("accepted invalid length")`, goEntry("probe.ts.Probe", "current"), goEntry("probe.ts.Probe", "bad"), length)
			got := runGoHIR(t, p, main)
			if !strings.Contains(got, "probe.ts:") || !strings.HasSuffix(got, "1/2\n") {
				t.Fatal(got)
			}
		})
	}
}

func TestGogenA26BooleanShadowNodeOracle(t *testing.T) {
	runMjsOracle(t, `export class Probe {static run(){const Boolean=(n)=>n===0;return String(Boolean(0))+":"+String(Boolean(1));}}`, true)
}
