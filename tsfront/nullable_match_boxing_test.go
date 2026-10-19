package tsfront

import (
	"strings"
	"testing"
)

// Keep the exact root parity source: its inferred coalesce/conditional context
// differs from a directly annotated Array<string> | null local.
func TestNullableMatchBoxingRootParity(t *testing.T) {
	source := `class Probe {
 static observe(flag:boolean):string {const picked=/^(foo)$/.exec("miss") ?? (flag ? ["x"] : null);const boxed:any=picked;return String(boxed===null)+"|"+String(boxed===undefined)+"|"+JSON.stringify({value:boxed});}
 static run():string {return Probe.observe(false)+";"+Probe.observe(true);}
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestNullableMatchBoxingEvaluationAndAliasNodeOracle(t *testing.T) {
	source := `class Holder {
 values:string[]=["x"]; flag:boolean;
 constructor(flag:boolean){this.flag=flag;}
 get fallback():string[]|null {Probe.fallbacks+=1;return this.flag?this.values:null;}
}
class Probe {
 static inputs:number=0; static fallbacks:number=0;
 static subject(value:string):string {Probe.inputs+=1;return value;}
 static inspect(value:unknown):string {const boxed:any=value;return String(boxed===null)+"|"+String(boxed===undefined)+"|"+String(Array.isArray(boxed))+"|"+JSON.stringify({value:boxed});}
 static observe(flag:boolean,input:string):string {
  const holder=new Holder(flag);
  const picked=/^(foo)$/.exec(Probe.subject(input)) ?? holder.fallback;
  const boxed:any=picked;const unknown:unknown=picked;
  const same=boxed===holder.values;holder.values.push("late");
  return Probe.inspect(unknown)+"|"+same;
 }
 static run():string {
  return Probe.observe(false,"miss")+";"+Probe.observe(true,"miss")+";"+Probe.observe(false,"foo")+";"+Probe.observe(true,"foo")+";"+Probe.inputs+"|"+Probe.fallbacks;
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestNullableMatchBoxingReturnsAndUndefinedNodeOracle(t *testing.T) {
	source := `class Probe {
 static returned(input:string):RegExpExecArray|null {return /^(foo)$/.exec(input);}
 static optional(flag:boolean):RegExpExecArray|null|undefined {return flag?Probe.returned("miss"):undefined;}
 static describe(value:unknown):string {const boxed:any=value;return (boxed===null?"N":boxed===undefined?"U":Array.isArray(boxed)?"A":"?")+"|"+JSON.stringify({value:boxed});}
 static run():string {
  const a:any=/^(foo)$/.exec("miss");
  const b:unknown=/^(foo)$/.exec("foo");
  const c:any=/^(foo)$/.exec("miss") ?? undefined;
  const d:any=Probe.returned("miss");const e:unknown=Probe.returned("foo");
  const f:any="miss".match(/^(foo)$/);const g:unknown="foo".match(/^(foo)$/);
  return Probe.describe(a)+";"+Probe.describe(b)+";"+Probe.describe(c)+";"+Probe.describe(d)+";"+Probe.describe(e)+";"+Probe.describe(f)+";"+Probe.describe(g)+";"+Probe.describe(Probe.optional(true))+";"+Probe.describe(Probe.optional(false));
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestNullableMatchBoxingMatchAliasNodeOracle(t *testing.T) {
	source := `class Probe {
 static returned():RegExpExecArray|null {return /^(foo)$/.exec("foo");}
 static run():string {const match=Probe.returned();const boxed:any=match;const unknown:unknown=match;if(match===null)return "bad";match[1]="late";return String(boxed===unknown)+"|"+String(Array.isArray(boxed))+"|"+JSON.stringify({value:boxed});}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestNullableMatchBoxingTypedNullAndUndefinedNodeOracle(t *testing.T) {
	source := `class Probe {
 static returned(flag:boolean):RegExpExecArray|null|undefined {return flag?/^(foo)$/.exec("miss"):undefined;}
 static observe(flag:boolean):string {const value=Probe.returned(flag);const boxed:any=value;return String(value===null)+"|"+String(value===undefined)+"|"+JSON.stringify({value:boxed});}
 static run():string {return Probe.observe(false)+";"+Probe.observe(true);}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestNullableMatchBoxingEmptyFallbackNodeOracle(t *testing.T) {
	source := `class Probe {
 static run():string {const missed=/^(foo)$/.exec("miss")??[];const matched=/^(foo)$/.exec("foo")??[];const a:any=missed;const b:unknown=matched;return JSON.stringify({value:a})+";"+JSON.stringify({value:b})+";"+String(missed.length)+"|"+String(matched.length);}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}
