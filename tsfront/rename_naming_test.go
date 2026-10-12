package tsfront

import (
	"fmt"
	"testing"
)

// A namespace class violates the default required class-name pattern even
// without renaming. Compare every decision with the JavaScript source oracle.
func TestRenameNamingConfigNodeOracle(t *testing.T) {
	source := `type PatternKind="required"|"forbidden";
 class BasicRuleConfig {severity=1;}
 abstract class Naming extends BasicRuleConfig {patternKind?:PatternKind="required";ignoreNames?:string[]=[];ignorePatterns?:string[]=[];}
 class Conf extends Naming {clas?:string="^ZC(L|X)";}
 class Validator {static violates(name:string,pattern:RegExp,params:Naming):boolean {
 if(params.ignoreNames && params.ignoreNames.indexOf(name.toUpperCase())>=0){return false;}
 for(const ignored of params.ignorePatterns || []) {if(new RegExp(ignored,"i").test(name)){return false;}}
 return !params.patternKind || params.patternKind==="required" ? pattern.test(name)===false : pattern.test(name)===true;
 }}
 class Rule {conf=new Conf();setConfig(conf:Conf):void{this.conf=conf;}getConfig():Conf{return this.conf;}
 run(name:string):string {
 if(this.conf.patternKind===undefined){this.conf.patternKind="required";}
 const config=this.getConfig();const abapType="clas";
 // @ts-ignore
 const pattern=config[abapType];
 if(pattern===undefined){return "missing";}
 const regex=new RegExp(pattern,"i");
 const issues:string[]=[];if(Validator.violates(name,regex,this.conf)){issues.push("object_naming");}
 return this.conf.patternKind+":"+pattern+":"+regex.test(name)+":"+issues.length;
 }}
 class Probe {static run():string {const r=new Rule();r.setConfig(JSON.parse('{"severity":1,"patternKind":"required","ignoreNames":[],"ignorePatterns":[],"clas":"^ZC(L|X)"}'));return r.run("/FOO/CL_BAR")+"|"+r.run("ZCL_BAR");}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

// Presence must come from the caller's record, including explicit undefined
// and fields not declared on the nominal layout. Native inheritance still uses
// its actual subclass table, and a receiver/key expression executes once.
func TestLiveConfigPropertyLookupNodeOracle(t *testing.T) {
	source := `class BasicRuleConfig {severity=1;}
 class Conf extends BasicRuleConfig {clas?:string="native";}
 class Derived extends Conf {extra="derived";}
 class Probe {
 static calls="";
 static receiver(conf:Conf):Conf {Probe.calls=Probe.calls+"r";return conf;}
 static key(key:string):string {Probe.calls=Probe.calls+"k";return key;}
 static read(conf:Conf,key:string):string {
 // @ts-ignore
 const value=Probe.receiver(conf)[Probe.key(key)];return value===undefined ? "absent" : ""+value;
 }
 static own(conf:Conf,key:string):boolean {return conf.hasOwnProperty(key);}
 static run():string {
 const raw=JSON.parse('{"clas":"parsed","extra":"kept"}');const conf:Conf=raw;
 const native=new Derived();
 const first=Probe.read(conf,"clas")+":"+Probe.read(conf,"extra")+":"+Probe.read(conf,"severity");
 conf.clas="written";const write=raw.clas;raw.clas="aliased";conf.severity=2;
 const next=Probe.read(conf,"clas")+":"+Probe.read(conf,"severity")+":"+Probe.own(conf,"severity");
 conf.clas=undefined;
 return first+"|"+write+"|"+next+"|"+Probe.read(conf,"clas")+":"+Probe.own(conf,"clas")+":"+Probe.own(conf,"extra")+":"+Probe.own(conf,"missing")+":"+Probe.own(conf,"toString")+"|"+Probe.read(native,"clas")+":"+Probe.read(native,"extra")+":"+Probe.read(native,"severity")+"|"+Probe.calls;
 }}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
