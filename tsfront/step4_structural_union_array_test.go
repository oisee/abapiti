package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const structuralUnionArraySource = `interface Supported {readonly name:string; readonly type:{k:"i"}; readonly unsupported?:undefined}
interface Refused extends Omit<Supported,"type"|"unsupported"> {readonly type?:{k:"i"}; readonly unsupported:string}
type Attribute=Supported|Refused;
interface ClassRecord {readonly name:string; readonly attributes:readonly Attribute[]}
function analyze(program:any):string {
 const classes=(program.classes??program) as readonly ClassRecord[];
 let result="";
 for(const cls of classes) for(const attr of cls.attributes) result+=attr.name+";";
 return result;
}
class Probe {static run():string {
 return analyze(JSON.parse('{"classes":[{"name":"C","attributes":[{"name":"X","type":{"k":"i"}},{"name":"Y","unsupported":"refused"}]}]}'));
}}`

func TestStep4StructuralUnionArrayNodeOracle(t *testing.T) {
	source := structuralUnionArraySource
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := structuralUnionProgram(t, source)
	got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
}

const structuralUnionArrayObservables = `interface Supported {name:string; readonly type:{k:"i"}; readonly unsupported?:undefined}
interface Refused extends Omit<Supported,"type"|"unsupported"> {readonly type?:{k:"i"}; readonly unsupported:string}
type Attribute=Supported|Refused;
interface Plain {name:string}
function isSupported(a:Attribute):a is Supported {Probe.trace++;return (a as {readonly unsupported?:string}).unsupported===undefined;}
interface Container {readonly rows:readonly (readonly Attribute[])[]; readonly plain:readonly Plain[]}
class Probe {
 static calls=0;static trace=0;
 static input():any {Probe.calls++;return JSON.parse('{"rows":[[{"name":"X","type":{"k":"i"}},{"name":"Y","unsupported":"refused"}]],"plain":[{"name":"plain"}]}');}
 static arm(a:Attribute):string {if(isSupported(a)) return a.type.k+":"+a.name;return a.unsupported+":"+(a.type===undefined);}
 static describe(a:Attribute):string {const unsupported=(a as {readonly unsupported?:string}).unsupported;return a.name+":"+(unsupported===undefined)+":"+(a.type?.k??"absent");}
 static run():string {
  const raw=Probe.input();const typed=raw as Container;
  const rows=typed.rows;const attrs=rows[0];const first=attrs[0];
  const identity=(rows===raw.rows)+":"+(attrs===raw.rows[0])+":"+(first===raw.rows[0][0])+":"+(first===attrs[0]);
  raw.rows[0][0].name="changed";raw.rows[0][0].unsupported=undefined;
  const before=Probe.describe(first)+"|"+Probe.describe(attrs[1]);
  const caller:any[]=raw.rows[0];caller.push({name:"Z",unsupported:"late"});
  (first as {name:string}).name="written";
  raw.plain[0].name="control";
  return identity+"|"+before+"|"+attrs.length+":"+Probe.describe(attrs[2])+"|"+raw.rows[0][0].name+"|"+typed.plain[0].name+"|"+Probe.calls+"|"+Probe.arm(first)+"|"+Probe.arm(attrs[1])+"|"+Probe.trace;
 }
 static invalid():void {const raw=Probe.input();const typed=raw as Container;const attrs=typed.rows[0];raw.rows[0][0]={name:"bad"};Probe.describe(attrs[0]);}
 static invalidUndefined():void {const raw=Probe.input();raw.rows[0][0]=undefined;const typed=raw as Container;const len=typed.rows[0].length;}
 static invalidAlias():void {const raw=Probe.input();const typed=raw as Container;const first=typed.rows[0][0];raw.rows[0][0].type=null;const name=first.name;}
}`

func TestStep4StructuralUnionArrayObservables(t *testing.T) {
	source := structuralUnionArrayObservables
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	for _, method := range []string{"invalid", "invalidUndefined", "invalidAlias"} {
		t.Run(method, func(t *testing.T) {
			main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("invalid late union mutation must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", method))
			if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), method)
		})
	}

}

func TestStep4StructuralUnionArrayInvalidMembers(t *testing.T) {
	for _, tc := range []struct{ name, payload string }{
		{"null", `null`}, {"primitive", `17`}, {"missing-name", `{"type":{"k":"i"}}`},
		{"wrong-name", `{"name":17,"type":{"k":"i"}}`}, {"no-arm", `{"name":"bad"}`},
		{"wrong-refusal", `{"name":"bad","unsupported":17}`}, {"null-refusal", `{"name":"bad","unsupported":null}`},
		{"null-type", `{"name":"bad","type":null}`}, {"primitive-type", `{"name":"bad","type":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(structuralUnionArraySource, `class Probe {static run():string {`, `class Probe {static invalid():void {analyze(JSON.parse('{"classes":[{"name":"C","attributes":[`+tc.payload+`]}]}'));} static run():string {`, 1)
			p := structuralUnionProgram(t, source)
			main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("invalid union must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
			if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", "X;Y;", "invalid")
		})
	}
}

const structuralUnionLiteralSource = `interface Left {readonly kind:"left"; readonly name:string; readonly optional?:string}
interface Right {readonly kind:"right"; readonly name:string; readonly other?:number}
interface Bag {readonly items:readonly (Left|Right)[]}
class Probe {
 static read(raw:any):string {const bag=raw as Bag;return bag.items[0].name;}
 static run():string {return Probe.read(JSON.parse('{"items":[{"kind":"left","name":"L"}]}'))+Probe.read(JSON.parse('{"items":[{"kind":"right","name":"R"}]}'));}
 static invalid():void {Probe.read(JSON.parse('{"items":[{"kind":"neither","name":"bad"}]}'));}
}`

func TestStep4StructuralUnionArrayLiteralDomains(t *testing.T) {
	p := structuralUnionProgram(t, structuralUnionLiteralSource)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "LR\n" {
		t.Fatal(got)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("invalid discriminant must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", "LR", "invalid")
}

func TestStep4StructuralUnionArrayNominalBoundary(t *testing.T) {
	source := `class Left {private brand=1;name="left";} class Right {private brand=2;name="right";}
 interface Bag {readonly items:readonly (Left|Right)[]}
 class Probe {static run():string {const items=[new Left(),new Right()];const raw:any={items};const bag=raw as Bag;return (bag.items[0]===items[0])+":"+(bag.items[1]===items[1])+":"+bag.items[0].name;}
 static invalid():void {const raw=JSON.parse('{"items":[{"name":"fake"}]}');const bag=raw as Bag;const first=bag.items[0];}}
 `
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "true:true:left\n" {
		t.Fatal(got)
	}
	main := fmt.Sprintf(`defer func(){if recover()==nil {panic("nominal union must refuse structural records")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", "true:true:left", "invalid", "cx_sy_move_cast_error")
}

const structuralUnionSourceDomains = `interface Left {readonly kind:"left";readonly tag:1;readonly flag:true;readonly mixed?:string|boolean;readonly maybe:string|undefined;readonly nullish:string|null}
interface Right {readonly kind:"right";readonly tag:2;readonly flag:false;readonly mixed?:string|boolean;readonly maybe:string|undefined;readonly nullish:string|null}
interface Bag {readonly items:readonly (Left|Right)[]}
class Probe {
 static read(raw:any):string {const bag=raw as Bag;return bag.items[0].kind;}
 static run():string {
 const raw=JSON.parse('{"items":[{"kind":"left","tag":1,"flag":true,"mixed":false,"maybe":"yes","nullish":null},{"kind":"right","tag":2,"flag":false,"mixed":"ok","maybe":"present","nullish":"yes"}]}');
 raw.items[0].maybe=undefined;
 const bag=raw as Bag;
 const left=bag.items[0];const right=bag.items[1];
 const native:Left={kind:"left",tag:1,flag:true,maybe:undefined,nullish:null,mixed:false};
 return left.kind+":"+left.tag+":"+left.flag+":"+typeof left.mixed+":"+(left.maybe===undefined)+":"+(left.nullish===null)+":"+(left.nullish===undefined)+"|"+right.kind+":"+right.tag+":"+right.flag+":"+typeof right.mixed+"|"+Probe.read({items:[native]});
 }
 static invalid():void {Probe.read(JSON.parse('{"items":[{"kind":"left","tag":1,"flag":true,"mixed":17,"maybe":"yes","nullish":null}]}'));}
 static invalidNumber():void {Probe.read(JSON.parse('{"items":[{"kind":"left","tag":3,"flag":true,"maybe":"yes","nullish":null}]}'));}
 static invalidBoolean():void {Probe.read(JSON.parse('{"items":[{"kind":"left","tag":1,"flag":false,"maybe":"yes","nullish":null}]}'));}
 static invalidMissing():void {Probe.read(JSON.parse('{"items":[{"kind":"left","tag":1,"flag":true,"nullish":null}]}'));}
 static invalidNative():void {const native:Left={kind:"left",tag:1,flag:true,maybe:undefined,nullish:null};const erased:any=native;erased.kind="bad";Probe.read({items:[native]});}
}`

func TestStep4StructuralUnionArraySourceDomains(t *testing.T) {
	p := structuralUnionProgram(t, structuralUnionSourceDomains)
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, structuralUnionSourceDomains).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	for _, method := range []string{"invalid", "invalidNumber", "invalidBoolean", "invalidMissing", "invalidNative"} {
		t.Run(method, func(t *testing.T) {
			main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("source domain must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", method))
			if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), method)
		})
	}
}

func TestStep4StructuralUnionArrayOptionalElements(t *testing.T) {
	source := `interface A {readonly kind:"a";readonly name:string} interface B {readonly kind:"b";readonly name:string}
 interface Bag {readonly items:readonly (A|B|undefined)[]}
 class Probe {static run():string {const raw=JSON.parse('{"items":[{"kind":"a","name":"A"},null,{"kind":"b","name":"B"}]}');raw.items[1]=undefined;const bag=raw as Bag;return bag.items[0]!.name+":"+(bag.items[1]===undefined)+":"+bag.items[2]!.name;}}
 `
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "A:true:B\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", "A:true:B", "")
}

func TestStep4StructuralUnionArrayTrapPolicy(t *testing.T) {
	p, ds, err := integerProbe(t, structuralUnionArraySource).LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "X;Y;\n" {
		t.Fatal(got)
	}
}

// Retain exact standalone probes for cross-backend review when requested by
// the gate driver; ordinary local test runs leave no repository artifacts.
func runStructuralUnionGo(t *testing.T, p *hir.Program, main string) string {
	t.Helper()
	base := os.Getenv("ABAPITI_TEST_OUT")
	var retained string
	if base != "" {
		var err error
		dir := filepath.Join(base, t.Name())
		if err = os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		retained, err = os.MkdirTemp(dir, "go-")
		if err != nil {
			t.Fatal(err)
		}
	}
	got := runGoHIR(t, p, main, func(files map[string]string) {
		if retained == "" {
			return
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(retained, name), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for name, source := range map[string]string{"main.go": "package main\nimport \"fmt\"\nfunc main(){" + main + "}\n", "go.mod": "module probe\n\ngo 1.26.0\n"} {
			if err := os.WriteFile(filepath.Join(retained, name), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if retained != "" {
		if err := os.WriteFile(filepath.Join(retained, "stdout.txt"), []byte(got), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return got
}

func structuralUnionProgram(t *testing.T, source string) *hir.Program {
	t.Helper()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		dir := filepath.Join(base, t.Name())
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		node, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(node))
		}
		for name, text := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"noEmit":true,"target":"ES2022"},"files":["probe.ts"]}`, "node.stdout.txt": string(node)} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
}

// The retained generator's IR.Type is metadata intersected with a union of
// object payloads. Its shallow domain is record storage, not an erased any.
func TestStep4StructuralUnionArrayIntersectionPayload(t *testing.T) {
	source := strings.ReplaceAll(structuralUnionArraySource, `{k:"i"}`, `AttrType`)
	source = `type AttrType={readonly label?:string}&({readonly k:"i"}|{readonly k:"c";readonly len:number});` + source
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "X;Y;\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", "X;Y;", "")
}

const structuralUnionOverlapSource = `interface A {readonly name:string; readonly a?:number}
interface B {readonly name:string; readonly b?:string}
interface Rows {readonly items:readonly (A|B)[]}
function read(x:A|B):string {
 if("b" in x) return "B:"+(x.b??"none");
 return "A:"+x.name;
}
class Probe {static run():string {
 const raw:any=JSON.parse('{"items":[{"name":"row","b":"bee"}]}');
 const view=raw as Rows;
 return read(view.items[0]);
}}`

func TestStep4StructuralUnionArrayOverlap(t *testing.T) {
	p := structuralUnionProgram(t, structuralUnionOverlapSource)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "B:bee\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", "B:bee", "")
}

func TestStep4StructuralUnionArrayOverlapAliases(t *testing.T) {
	source := strings.Replace(structuralUnionOverlapSource, `return read(view.items[0]);`, `const first=view.items[0];const same=first===raw.items[0];const initial=read(first);raw.items[0].b="changed";const changed=read(first);raw.items[0].b=undefined;const absent=read(first);delete raw.items[0].b;return same+"|"+initial+"|"+changed+"|"+absent+"|"+read(first)+"|"+(first===view.items[0]);`, 1)
	source = strings.Replace(source, `class Probe {static run():string {`, `class Probe {static invalid():void {const raw=JSON.parse('{"items":[{"name":"row","b":17}]}');const view=raw as Rows;read(view.items[0]);} static run():string {`, 1)
	p := structuralUnionProgram(t, source)
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("invalid narrowed member must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "invalid")
}
