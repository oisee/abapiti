package tsfront

import (
	"fmt"
	"testing"
)

func TestAssignmentPrimitiveConfigFieldsNodeOracle(t *testing.T) {
	source := `class Conf {length:number=120;label:string="old";enabled:boolean=false;}
 class Probe {static trace="";static conf=new Conf();
 static receiver():Conf {Probe.trace=Probe.trace+"L";return Probe.conf;}
 static value():number {Probe.trace=Probe.trace+"R";return 400;}
 static accept(value:any):string {return String(value);}
 static run():string {const n=Probe.accept(Probe.receiver().length=Probe.value());
 const s=Probe.accept(Probe.conf.label="new");const b=Probe.accept(Probe.conf.enabled=true);
 return n+":"+Probe.conf.length+":"+s+":"+Probe.conf.label+":"+b+":"+Probe.conf.enabled+":"+Probe.trace;}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestAssignmentPrimitiveConfigViewNodeOracle(t *testing.T) {
	source := `class Conf {length:number=120;max:number|undefined=10;}
 class Rule {conf=new Conf();setConfig(c:Conf):void {this.conf=c;}
 read(n:number):boolean {let max=this.conf.max;if(max===undefined || max<1){max=10;}return n>this.conf.length;}}
 class Probe {static accept(value:any):Rule {const rule=new Rule();rule.setConfig(value);return rule;}
 static required():number {return Probe.accept(400).conf.length;}
 static write():void {Probe.accept(400).conf.length=1;}
 static run():string {const conf=new Conf();const rule=Probe.accept(conf.length=400);const value:any=rule.conf;
 return conf.length+":"+rule.read(12)+":"+rule.read(500)+":"+(value===400)+":"+Probe.accept(conf).read(500);}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	for _, method := range []string{"required", "write"} {
		t.Run(method, func(t *testing.T) {
			main := fmt.Sprintf(`defer func(){switch recover().(type) {case trap,rangeFault: default: panic("unsupported primitive config access escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", method))
			if got := runGoHIR(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
		})
	}

}
