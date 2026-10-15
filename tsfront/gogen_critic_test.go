package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestGogenObjectParameterNullish(t *testing.T) {
	for _, pattern := range []string{"{}", "{a}"} {
		for _, absent := range []string{"undefined", "null"} {
			t.Run(pattern+absent, func(t *testing.T) {
				source := `function f(` + pattern + `:{a?:number}|null|undefined):number {return 7;} class Probe {static run():number {return f(` + absent + `);}}`
				p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
				got := runGoHIR(t, p, fmt.Sprintf(`defer func(){r:=recover();if r==nil {panic("missing TypeError")};fmt.Printf("%%#v\n",r)}();%s()`, goEntry("probe.ts.Probe", "run")))
				if !strings.Contains(got, "builtin.TypeError") {
					t.Fatalf("expected JS TypeError payload, got %q", got)
				}
				if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "probe.ts:1:") {
					t.Fatalf("expected located ABAP refusal, got %v", err)
				}
			})
		}
	}
}

func TestGogenBoxedSet(t *testing.T) {
	source := `class Probe {static run():string {const nan=parseFloat("NaN");const input:(number|undefined)[]=[1,1,nan,nan,-0,0,undefined,undefined];const s=new Set(input);return s.size+":"+s.has(1)+":"+s.has(nan)+":"+s.has(-0)+":"+s.has(0)+":"+s.has(input[6]);}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "4:true:true:true:true:true\n" {
		t.Fatal(got)
	}
	if node := conversionOracle(t, source); node != "4:true:true:true:true:true\n" {
		t.Fatal(node)
	}
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "probe.ts:1:") || !strings.Contains(err.Error(), "SameValueZero") {
		t.Fatalf("expected located ABAP boxed Set refusal, got %v", err)
	}
}

func TestGogenNullableObjectParameterPresent(t *testing.T) {
	source := `function f({a}: {a:number}|null|undefined):number {return a;} class Probe {static run():number {return f({a:3});}}`
	gogenOracle(t, source, "3", "RequireObjectCoercible")
}

func TestGogenPrimitivePushSpreadCheckedProducer(t *testing.T) {
	source := `class Item {a=1;} class Probe {
 static copy(xs:number[]):number {const a:number[]=[];a.push(2,...xs);return a.length;}
 static run():string {const xs=new Array<Item>(1);const out:Item[]=[];out.push(...xs);return Probe.copy([1,3])+":"+(out[0]===undefined);}
 static sparse():void {const xs=new Array<number>(1);Probe.copy(xs);}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf(`fmt.Println(%s());defer func(){r,ok:=recover().(trap);if !ok {panic("missing located sparse refusal")};fmt.Println(r.Source)}();%s()`, goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "sparse")))
	if !strings.HasPrefix(got, "3:true\nprobe.ts:2:") || !strings.Contains(got, "cannot represent undefined") {
		t.Fatal(got)
	}
	if node := conversionOracle(t, source); node != "3:true\n" {
		t.Fatal(node)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", "3:true", "sparse", "exception.unexecuted")
}

func TestGogenObjectParameterTypeErrorCatch(t *testing.T) {
	source := `function f({}: {a?:number}|undefined):number {return 7;} class Probe {static run():string {try {return "bad:"+f(undefined);} catch(e) {return "caught:"+(e instanceof TypeError)+":"+(e instanceof Error);}}}`
	gogenOracle(t, source, "caught:true:true", "RequireObjectCoercible")
}

func TestGogenAnyObjectParameterNullish(t *testing.T) {
	source := `function f({a}:any):number {return 7;} class Probe {static run():string {let caught=0;try {f(undefined);} catch(e) {if(e instanceof TypeError){caught++;}}try {f(null);} catch(e) {if(e instanceof TypeError){caught++;}}return ""+caught;}}`
	gogenOracle(t, source, "2", "RequireObjectCoercible")
}
