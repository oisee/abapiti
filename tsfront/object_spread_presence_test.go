package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// Like keyword_case's lower configs, these literals spread inherited defaults
// before overriding fields and crossing an erased interface config boundary.
func TestObjectSpreadConfigPresenceNodeOracle(t *testing.T) {
	source := `class Base {exclude?:string[]=[];severity?:string="Error";}
 class Conf extends Base {style="upper";ignoreBoundaries=false;maxIssuesPerFile:number|undefined=10;}
 interface Slot {setConfig(config:any):void;}
 class Rule implements Slot {config=new Conf();setConfig(config:Conf):void {this.config=config;}}
 class Probe {
 static trace="";
 static source():Conf {Probe.trace=Probe.trace+"s";return new Conf();}
 static override():string {Probe.trace=Probe.trace+"o";return "last";}
 static maybe(present:boolean):Conf|undefined {return present ? new Conf() : undefined;}
 static run():string {
 const ordered={...Probe.source(),style:Probe.override()};
 const absentSpread={...Probe.maybe(false),style:"missing"};
 const presentSpread={...Probe.maybe(true),style:"present"};
 const native=new Conf();
 const configs=[{...native,style:"lower"},{...new Conf(),style:"lower",ignoreBoundaries:true}];
 const rows=[{abap:'\nclass "quoted".\n',cnt:1},{abap:"interface zif.\n",cnt:2}];
 const changed=rows.map((row,index)=>index===1 ? {...row,cnt:0} : row);
 let result="";
 for(const config of configs) {
 const rule=new Rule();const slot:Slot=rule;slot.setConfig(config);
 result=result+rule.config.style+":"+rule.config.ignoreBoundaries+":"+rule.config.maxIssuesPerFile+":"+rule.config.severity+":"+rule.config.exclude!.length+":"+Object.keys(config).join(",")+"|";
 }
 const absent={...native,maxIssuesPerFile:undefined};
 const copied={...absent};
 return Probe.trace+":"+ordered.style+":"+Object.keys(absentSpread).join(",")+":"+Object.keys(presentSpread).join(",")+"|"+result+(configs[0].exclude===native.exclude)+":"+Object.keys(copied).join(",")+":"+(copied.maxIssuesPerFile===undefined)+":"+changed.map(row=>'"'+row.abap+'" '+row.cnt).join("|");
 }} `
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "spread-config", strings.TrimSpace(want), false)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
