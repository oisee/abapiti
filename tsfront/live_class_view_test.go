package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
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
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if dir := fixtureOutputDir(t); dir != "" {
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
			if _, err := emitABAP(t, p); err != nil {
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

func TestLiveTypedArrayNodeOracle(t *testing.T) {
	source := `class Conf {names?:string[];}
 interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {
 static run():string {
 const input=JSON.parse('{"names":["a","b"]}');const rule=new Rule();const slot:Slot=rule;slot.setConfig(input);
 const typed=rule.conf.names!;const caller:any[]=input.names;
 const same=typed===input.names;caller.push("c");const seen=typed.length+":"+typed[2];
 typed.push("d");typed[0]="A";const first=caller[0];caller[1]="B";
 const removed=typed.splice(1,1,"x");caller.splice(2,1,"y");
 const other=rule.conf.names!;other.push("z");
 return same+"|"+seen+"|"+first+"|"+removed.join(",")+"|"+typed.join(",")+"|"+caller.length+":"+caller[0]+":"+caller[1]+":"+caller[2]+":"+caller[3]+":"+caller[4];
 }
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	exportLiveViewABAP(t, p, "typed-array", strings.TrimSpace(string(want)), true)
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestLiveTypedArrayPlainNodeOracle(t *testing.T) {
	source := `class Conf {names?:string[];} interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {static run():string {const caller:any[]=["a","b"];const input={names:caller};const r=new Rule();const s:Slot=r;s.setConfig(input);const typed=r.conf.names!;caller.push("c");typed[0]="A";typed.splice(1,1,"B");const viewed:any=typed;return (viewed===caller)+":"+typed.join(",")+":"+caller[0]+":"+caller[1]+":"+caller.length;}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "plain-array", strings.TrimSpace(string(want)), true)
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestLiveClassViewOwnKeysNodeOracle(t *testing.T) {
	source := `class Conf {optional?:string;}
 interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {
 static keys(text:string):string {
 const input=JSON.parse(text);const r=new Rule();const slot:Slot=r;slot.setConfig(input);
 let iter="";for(const key in r.conf) {iter=iter+key+",";}
 const keys=Object.keys(r.conf).join(",");let entries="";for(const [key,value] of Object.entries(r.conf)) {entries=entries+key+":"+value+",";}
 r.conf.optional="live";
 return keys+"|"+iter+"|"+entries+"|"+Object.keys(r.conf).join(",");
 }
 static run():string {return Probe.keys('{}')+"\n"+Probe.keys('{"extra":false,"10":"ten","2":"two","01":"one","4294967295":"last","4294967294":"max","4294967296":"over","0":"zero","00":"double","":"empty","é":"unicode"}')+"\n"+Probe.keys('{"optional":"yes","extra":true}');}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	exportLiveViewABAP(t, p, "own-keys", strings.TrimSpace(string(want)), false)
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestLiveTypedArrayElementRefusals(t *testing.T) {
	for _, text := range []string{`["ok",17]`, `["ok",null]`, `["ok",{}]`} {
		source := `class Conf {names?:string[];} interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {static bad():number {const input=JSON.parse('{"names":` + text + `}');const r=new Rule();const s:Slot=r;s.setConfig(input);return r.conf.names!.length;}}`
		p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
		main := fmt.Sprintf(`defer func(){switch recover().(type) {case trap,rangeFault:default:panic("invalid live element must explicitly refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "bad"))
		if got := runGoHIR(t, p, main); got != "refused\n" {
			t.Fatal(got)
		}
	}
	source := `class Conf {names?:string[];} interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {static bad():string {const input=JSON.parse('{"names":["ok"]}');const r=new Rule();const s:Slot=r;s.setConfig(input);const typed=r.conf.names!;const caller:any[]=input.names;caller[0]=17;return typed[0];}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	main := fmt.Sprintf(`defer func(){switch recover().(type) {case trap,rangeFault:default:panic("later caller write must explicitly refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "bad"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func exportLiveViewABAP(t *testing.T, p *hir.Program, suffix, want string, refusal bool) {
	t.Helper()
	root := fixtureOutputDir(t)
	if root == "" {
		return
	}
	dir := filepath.Join(root, suffix)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	cls := names.Get("probe.ts.Probe")
	method := names.Get("member.run")
	body := "DATA actual TYPE string.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\n" + strings.Join(abapStringBuild("expected", "ch", want), "\n") + "\n" + fmt.Sprintf("actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\n", cls, method)
	if refusal {
		body = fmt.Sprintf("TRY.\n%s=>%s( ).\ncl_abap_unit_assert=>fail( ).\nCATCH %s.\nENDTRY.\n", cls, method, names.Get("view.refusal"))
	}
	files[cls+".clas.testclasses.abap"] = "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\n" + body + "ENDMETHOD.\nENDCLASS.\n"
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLiveClassViewJSONNodeOracle(t *testing.T) {
	source := `class Conf {optional?:string;value=1;} interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {static run():string {const input=JSON.parse('{"extra":false,"10":"ten","2":"two"}');const r=new Rule();const s:Slot=r;s.setConfig(input);const before=JSON.stringify(r.conf);r.conf.optional="live";r.conf.value=2;return before+"|"+JSON.stringify(r.conf)+"|"+JSON.stringify(input);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "json.stringifySubset") {
		t.Fatalf("ABAP must explicitly refuse subset JSON: %v", err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestLiveTypedArrayNestedNodeOracle(t *testing.T) {
	source := `class Conf {names?:string[][];} interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}}
 class Probe {static run():string {const input=JSON.parse('{"names":[["a"]]}');const r=new Rule();const s:Slot=r;s.setConfig(input);const typed=r.conf.names!;const inner=typed[0];const viewed:any=inner;const caller:any[]=input.names[0];inner.push("b");caller[0]="A";const seen=new Set<string>();seen.add(inner[0]);const pieces="a-b".split("-");let count=seen.size;if(pieces[2]) {count=99;}return (viewed===input.names[0])+":"+typed[0].join(",")+":"+caller.length+":"+count;}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStructuralOwnKeysLiveView(t *testing.T) {
	source := `class Conf {value=0; missing?:string; added?:string;}
 interface Slot {setConfig(conf:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(conf:Conf):void {this.conf=conf;}write():void {this.conf.added="new";}}
 class Probe {static run():string {
 const input:{value:number; missing?:string; added?:string}={missing:undefined,value:1};
 const rule=new Rule();const slot:Slot=rule;slot.setConfig(input);
 const before=Object.keys(rule.conf).join(",");rule.write();
 return before+"|"+Object.keys(rule.conf).join(",")+"|"+Object.keys(input).join(",")+"|"+input.added+"|"+JSON.stringify(input)+"|"+JSON.stringify(rule.conf);
 }} `
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "json.stringifySubset") {
		t.Fatalf("expected explicit ABAP JSON refusal, got %v", err)
	}
	got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())")
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStructuralOwnKeysEvaluationOrder(t *testing.T) {
	source := `interface Row {a:number;b:number;} class Probe {static log="";static tick(n:number):number {Probe.log=Probe.log+n;return n;}
 static pick(row:Row):Row {Probe.log=Probe.log+"p";return row;}
 static run():string {const first:Row={a:Probe.tick(1),b:Probe.tick(2)};const second:Row={b:Probe.tick(3),a:Probe.tick(4)};Probe.pick(first).a=5;return Object.keys(first).join(",")+"|"+Object.keys(second).join(",")+"|"+Probe.log;}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())")
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestNativeConfigOwnKeysNodeOracle(t *testing.T) {
	source := `class Base {exclude:string[]=[];severity="Error";}
 class Conf extends Base {callMethod=true;callFunction=true;internalTable=true;}
 class Probe {
 static configure(config:any,selected:string):string {
 let order="";for(const key in config){order=order+key+",";if(typeof config[key]==="boolean"){config[key]=false;}}
 config[selected]=true;
 return Object.keys(config).join(",")+"|"+order+"|"+config.severity+"|"+"|"+config.callMethod+":"+config.callFunction+":"+config.internalTable;
 }
 static run():string {const native=new Conf();const exclude=native.exclude;const first=Probe.configure(native,"callMethod");const alias:any=native;
 const second=Probe.configure(alias,"internalTable");return first+"\n"+second+"\n"+native.callMethod+":"+native.callFunction+":"+native.internalTable+"|"+(native.exclude===exclude)+":"+native.exclude.length;}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "native-keys", strings.TrimSpace(want), false)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestPlainConfigOwnKeysNodeOracle(t *testing.T) {
	source := `class Conf {a=true;optional?:string;}
 interface Slot {setConfig(value:any):void;}
 class Rule implements Slot {conf=new Conf();setConfig(value:Conf):void {this.conf=value;}}
 class Probe {
 static trace="";
 static value(label:string):boolean {Probe.trace=Probe.trace+label;return true;}
 static run():string {
 const input={b:Probe.value("b"),a:Probe.value("a")};const alias:any=input;
 const before=Object.keys(input).join(",");const rule=new Rule();const slot:Slot=rule;slot.setConfig(input);
 input.a=false;rule.conf.optional="live";alias["2"]=true;
 let iter="";for(const key in alias){iter=iter+key+",";}
 return Probe.trace+"|"+before+"|"+Object.keys(rule.conf).join(",")+"|"+iter+"|"+rule.conf.a+"|"+alias.optional+"|"+(alias["2"]===true);
 }} `
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "plain-keys", strings.TrimSpace(want), false)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestTaggedBooleanEqualityNodeOracle(t *testing.T) {
	source := `class Probe {
 static reads=0;
 static value(value:any):any {Probe.reads=Probe.reads+1;return value;}
 static run():string {
 const t:any=true,f:any=false,u:any=undefined,o:any={};
 return (t===true)+":"+(true===t)+":"+(f===false)+":"+(false===f)+":"+(t!==true)+":"+(true!==f)+":"+(u===false)+":"+(o===true)+":"+(Probe.value(t)===true)+":"+(false===Probe.value(f))+":"+Probe.reads;
 }} `
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	exportLiveViewABAP(t, p, "tagged-bool", strings.TrimSpace(want), false)
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
