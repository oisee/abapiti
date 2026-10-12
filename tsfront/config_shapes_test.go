package tsfront

import (
	"fmt"
	"testing"
)

func TestConfigShapeWritesNodeOracle(t *testing.T) {
	source := `class Conf {severity=1;}
 interface Slot {setConfig(c:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(c:Conf):void {this.conf=c;}}
 class Probe {static run():string {
 const indexed:{[key:string]:any}={};indexed.keyword_case="upper";indexed.sequential_blank=undefined;
 const original={severity:2};const rule=new Rule();const slot:Slot=rule;slot.setConfig(original);
 const erased:any=rule.conf;const nested:any={};erased.config=nested;erased.keyword_case="upper";erased.patternKind="required";
 (rule.conf as any).extra="added";erased["severity"]=3;const before=Object.keys(erased).join(",");erased.keyword_case="lower";
 const parsed=JSON.parse('{"severity":4}');slot.setConfig(parsed);const other:any=rule.conf;
 other.applyUnspecifiedRules=true;rule.conf.severity=5;other.applyUnspecifiedRules=undefined;
 return (erased.config===nested)+":"+Object.keys(indexed).join(",")+":"+(indexed.sequential_blank===undefined)+":"+(erased===original)+":"+original.severity+":"+before+":"+Object.keys(erased).join(",")+":"+erased.keyword_case+":"+(other===parsed)+":"+parsed.severity+":"+Object.keys(parsed).join(",")+":"+(parsed.applyUnspecifiedRules===undefined);
 }}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestLiteralErasedPrimitiveAbsenceNodeOracle(t *testing.T) {
	source := `interface Row {low:string;high:string;description:string;}
 class Probe {static read(input:any):Row {return {description:input.description,low:input.low,high:input.high};}
 static run():string {
 const input=JSON.parse('{"low":"a"}');const row=Probe.read(input);const erased:any=row;
 const empty: {value?:string}={};const explicit:{value?:string}={value:undefined};
 const before=Object.keys(row).join(",");const absent=row.high===undefined;
 row.high="z";const written=row.high;row.high=undefined as any;
 return absent+":"+before+":"+Object.keys(row).join(",")+":"+written+":"+(erased.high===undefined)+":"+Object.keys(empty).length+":"+Object.keys(explicit).join(",");
 }}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestNominalConfigLiteralNodeOracle(t *testing.T) {
	source := `class Base {severity=1;} abstract class Conf extends Base {patternKind?:string="required";}
 class Probe {static check(c:Conf):string {return (c.patternKind===undefined)+":"+Object.keys(c).join(",");}
 static run():string {return Probe.check({patternKind:"forbidden"})+"|"+Probe.check({patternKind:undefined})+"|"+Probe.check({});}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestDefaultFillingConfigViewsNodeOracle(t *testing.T) {
	source := `type Style="upper"|"lower";
 class Conf {style:Style="upper";ignoreExceptions:boolean=true;}
 class Rule {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;if(this.conf===undefined){this.conf=new Conf();}if(this.conf.style===undefined){this.conf=new Conf();}if(this.conf.ignoreExceptions===undefined){this.conf.ignoreExceptions=new Conf().ignoreExceptions;}}}
 class Probe {static style(input:any):string {const style:Style=input ? input["style"] : "upper";return style==="lower" ? "lower" : "upper";}
 static accept(input:any):Conf {const rule=new Rule();rule.setConfig(input);return rule.conf;}
 static run():string {
 const partial=JSON.parse('{"style":"lower"}');const viewed=Probe.accept(partial);
 const native=new Conf();const same=Probe.accept(native)===native;
 return Probe.accept(true).style+":"+Probe.accept(undefined).style+":"+Probe.accept(7).style+":"+Probe.accept("x").style+":"+(viewed===partial)+":"+partial.ignoreExceptions+":"+Object.keys(partial).join(",")+":"+same+":"+Probe.style(true)+":"+Probe.style(undefined)+":"+Probe.style(partial);
 }}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
