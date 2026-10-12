package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestErasedStringAssertionStorageNodeOracle(t *testing.T) {
	source := `class Probe {
 static put(value:any):any {const result:any={};result.value=value as string;return result;}
 static run():string {const absent=Probe.put(undefined);const nested=JSON.parse('{"x":1}');const row=Probe.put(nested);nested.x=2;return (absent.value===undefined)+":"+(row.value===nested)+":"+row.value.x+":"+Object.keys(absent).join(",");}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestNonfiniteNumberComparisonNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const nan=parseFloat("");const inf=parseFloat("Infinity!");const neg=parseFloat("-Infinity");
 const erased:any=nan;const values=[nan,inf,neg];
 return isNaN(nan)+":"+(nan===nan)+":"+(nan!==nan)+":"+(nan<0)+":"+(nan>=0)+":"+(inf>1e300)+":"+(neg<0)+":"+(nan ? "yes":"no")+":"+(erased ? "yes":"no")+":"+isNaN(nan+1)+":"+String(-inf)+":"+String(parseInt("",10))+":"+String(parseInt("9".padStart(400,"9"),10))+":"+JSON.stringify(values)+":"+String(parseInt(JSON.parse("{}"),10))+":"+String(parseFloat(JSON.parse("{}")));
 }} `
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestSiblingNominalAssertionChecksRuntime(t *testing.T) {
	source := `class Base {value=1;} class Left extends Base {} class Right extends Base {extra=2;}
 class Probe {static wrong(input:Left):Right {return input as Right;}static run():void {Probe.wrong(new Left());}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if x:=recover();x!=nil {fmt.Println(x)}}();%s()`, goEntry("probe.ts.Probe", "run")))
	if !strings.Contains(got, "value outside supported") {
		t.Fatalf("true nominal mismatch escaped: %q", got)
	}
}

func TestLiveErasedRowArrayNodeOracle(t *testing.T) {
	source := `interface Row {name?:string;} class Probe {
 static view(input:any[]):Row[] {return input;}
 static run():string {const input:any[]=JSON.parse('[{"name":"a"},{}]');const rows=Probe.view(input);input[0].name="b";const before=rows[0].name;rows[0].name="c";rows.push({name:"d"});return before+":"+input[0].name+":"+(rows[1].name===undefined)+":"+input.length+":"+(rows===input);}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestCommonAncestorArgumentAssertionNodeOracle(t *testing.T) {
	source := `interface ObjectLike {read():number;}
 abstract class Base implements ObjectLike {value=1;read():number{return this.value;}}
 class Child extends Base {extra=2;} class Sibling extends Base {read():number{return this.value+10;}}
 class Probe {static use(input:Base):number {return input.read();}
 static common(input?:ObjectLike):number {const view=input as Child|undefined;if(view===undefined){return -1;}return Probe.use(view);}
 static run():string {return Probe.common(new Child())+":"+Probe.common(new Sibling())+":"+Probe.common();}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestPlainRecordStringNodeOracle(t *testing.T) {
	source := `class Probe {static count=0;static once(value:any):any {Probe.count=Probe.count+1;return value;}
 static run():string {const parsed=JSON.parse('{"nested":{"value":1}}');const native:any={};native.value=parsed;return String(Probe.once(parsed))+":"+("value="+native)+":"+String(undefined)+":"+String(null)+":"+Probe.count;}
 static hooks():string {return String(JSON.parse('{"toString":null}'));}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok {panic("hook boundary escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "hooks")))
	if got != "refused\n" {
		t.Fatal(got)
	}
}
