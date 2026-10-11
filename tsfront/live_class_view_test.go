package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestLiveClassViewNodeOracle(t *testing.T) {
	source := `class BasicConf {value=1; optional?:string="native"; items?:number[]; static made=0; constructor(){BasicConf.made=BasicConf.made+1;}}
 class RuleConf extends BasicConf {readValue():number{return this.value;}}
 interface IRule {setConfig(conf:any):void;getConfig():void;}
 class Rule implements IRule {
 private conf=new RuleConf();
 setConfig(conf:RuleConf):void {this.conf=conf;}
 getConfig():RuleConf {return this.conf;}
 read():number {return this.conf.value;}
 write(value:number):void {this.conf.value=value;}
 writeOptional(value:string):void {this.conf.optional=value;}
 writeItems(value:number[]):void {this.conf.items=value;}
 }
 class Probe {
 static native():string {const conf=new RuleConf();const r=new Rule();const s:IRule=r;s.setConfig(conf);const current:any=r.getConfig();conf.value=2;r.write(r.read()+1);return (current===conf)+":"+conf.value+":"+(current instanceof RuleConf);}
 static plain():string {const conf={value:1,extra:"kept"};const r=new Rule();const s:IRule=r;s.setConfig(conf);const current:any=r.getConfig();conf.value=2;const seen=r.read();r.write(3);const absent=r.getConfig().optional===undefined;r.writeOptional("live");return absent+":"+current.optional+":"+(current===conf)+":"+seen+":"+conf.value+":"+conf.extra+":"+(current instanceof RuleConf)+":"+(r.getConfig().optional===undefined);}
 static parsed():string {const conf=JSON.parse('{"value":1,"extra":"kept"}');const r=new Rule();const other=new Rule();const s:IRule=r;const s2:IRule=other;s.setConfig(conf);s2.setConfig(conf);const current:any=r.getConfig();conf["value"]=2;const seen=r.read();other.write(3);return (current===conf)+":"+seen+":"+r.read()+":"+conf.value+":"+(r.getConfig()===other.getConfig())+":"+conf.extra+":"+(current instanceof RuleConf)+":"+(r.getConfig().optional===undefined);}
 static array():string {const conf={value:1,items:[1]};const r=new Rule();const s:IRule=r;s.setConfig(conf);conf.items[0]=2;const seen=r.getConfig().items![0];const replacement=[3];r.writeItems(replacement);return (conf.items===replacement)+":"+seen+":"+conf.items[0];}
 static missing():string {const conf=JSON.parse('{"extra":"only"}');const r=new Rule();const s:IRule=r;s.setConfig(conf);const current:any=r.getConfig();return (current===conf)+":"+(current.value===undefined)+":"+(current.optional===undefined)+":"+current.extra;}
 static run():string {return Probe.native()+"|"+Probe.plain()+"|"+Probe.parsed()+"|"+Probe.array()+"|"+Probe.missing()+"|"+BasicConf.made;}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, names, err := abap.EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("ABAPITI_TEST_OUT"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		cls := names.Get("probe.ts.Probe")
		method := names.Get("member.run")
		files[cls+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nactual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\nENDMETHOD.\nENDCLASS.\n", cls, method, strings.TrimSpace(string(want)))
		for name, text := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestLiveClassViewRefusesPrototypeStorage(t *testing.T) {
	for _, tc := range []struct{ name, declaration string }{
		{"method", `class Conf {value=1;self():Conf{return this;}}`},
		{"used-reader", `class Conf {value=1;readValue():number{return this.value;}} class User {static use(conf:Conf):number{return conf.readValue();}}`},
		{"getter", `class Conf {get value():number{return 1;}}`},
		{"private", `class Conf {private value=1;}`},
		{"hash-private", `class Conf {#value=1;}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.declaration + `interface Slot {setConfig(conf:any):void;getConfig():void;}
  class Rule implements Slot {private conf=new Conf();setConfig(conf:Conf):void{this.conf=conf;}getConfig():Conf{return this.conf;}}
  class Probe {static accept(conf:any):Conf {const r=new Rule();const s:Slot=r;s.setConfig(conf);return r.getConfig();}
  static native():boolean {const conf=new Conf();return Probe.accept(conf)===conf;}
  static invalid():void {Probe.accept({value:1});}}`
			want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.native())")`, source).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			if _, err := abap.Emit(p); err != nil {
				t.Fatal(err)
			}
			if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "native"))); got != string(want) {
				t.Fatalf("Go %q Node %q", got, want)
			}
			main := fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok {panic("structural behaviour must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
			if got := runGoHIR(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
		})
	}
}
