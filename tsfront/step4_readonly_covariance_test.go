package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const readonlyCovarianceTypes = `export interface IRMethod {readonly kind:"method";readonly name:string;readonly extra:boolean}
export interface Method {readonly kind:"method";readonly name:string}
export interface Unsupported {readonly kind:"unsupported";readonly reason:string}
export interface IRClass {readonly signatures:ReadonlyMap<string,IRMethod>;readonly methods:ReadonlySet<IRMethod>;readonly rows:readonly IRMethod[]}
export interface Class {readonly signatures?:ReadonlyMap<string,Method|Unsupported>;readonly methods:ReadonlySet<Method|Unsupported>;readonly rows:readonly (Method|Unsupported)[]}
`
const readonlyCovarianceBody = `const signatures=cls.signatures;if(!signatures)return "absent";let out="";for(const [k,v] of signatures)out+=k+":"+v.kind+",";
 const first=signatures.get("a");
 const foreign={kind:"method",name:"foreign"};
 return [...signatures.values()].map(v=>v.kind).join(",")+":"+(first?.kind??"none")+":"+signatures.has("a")+":"+signatures.has("missing")+":"+signatures.size+":"+out+":"+[...cls.methods].map(v=>v.kind).join(",")+":"+cls.methods.has(foreign)+":"+cls.methods.size+":"+cls.rows.map(v=>v.kind).join(",");`

func TestStep4ReadonlyCovarianceTS(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
 function read(cls:Class):string {` + readonlyCovarianceBody + `}
 class Probe {static run():string {const a:IRMethod={kind:"method",name:"a",extra:true};const signatures=new Map<string,IRMethod>([["a",a]]);const methods=new Set<IRMethod>([a]);const rows:IRMethod[]=[a];const ir:IRClass={signatures,methods,rows};const cls:Class=ir;const before=read(cls);const b:IRMethod={kind:"method",name:"b",extra:false};signatures.set("b",b);methods.add(b);rows.push(b);return before+"|"+read(cls);}}
 `
	// Keep foreign's literal narrow for strict TS, matching JS's JSDoc carrier.
	source = replaceCovarianceForeign(source)
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}
func replaceCovarianceForeign(s string) string {
	return strings.ReplaceAll(s, `const foreign={kind:"method",name:"foreign"}`, `const foreign:Method={kind:"method",name:"foreign"}`)
}

func TestStep4ReadonlyCovarianceJS(t *testing.T) {
	mjs := `/** @param {import('./types.ts').Class} cls @returns {string} */
 function read(cls) {` + strings.ReplaceAll(readonlyCovarianceBody, `const foreign=`, `/** @type {import("./types.ts").Method} */ const foreign=`) + `}
 export class Probe {static run(){
 /** @type {import('./types.ts').IRMethod} */ const a={kind:"method",name:"a",extra:true};
 const signatures=new Map([["a",a]]),methods=new Set([a]),rows=[a];
 /** @type {import('./types.ts').IRClass} */ const ir={signatures,methods,rows};
 /** @type {import('./types.ts').Class} */ const cls=ir;
 const before=read(cls);signatures.set("b",a);methods.add({kind:"method",name:"b",extra:false});rows.push(a);return before+"|"+read(cls);}}
 `
	readonlyStrictCheck(t, map[string]string{"types.ts": readonlyCovarianceTypes, "probe.mjs": mjs})
	runMjsOracleTypes(t, readonlyCovarianceTypes, mjs, false)
}

func readonlyStrictCheck(t *testing.T, sources map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := `{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowJs":true,"checkJs":true,"noEmit":true,"allowImportingTsExtensions":true},"include":["*.ts","*.mjs"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if diags := p.CheckerDiagnostics(t.Context()); len(diags) != 0 {
		t.Fatalf("strict checker: %v", diags)
	}
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out := filepath.Join(base, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range sources {
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(out, "tsconfig.json"), []byte(cfg), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func readonlyGoOracle(t *testing.T, source string, refuseABAP bool) {
	t.Helper()
	want := nodePlatformOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if refuseABAP {
		if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "ABAP cannot share storage for covariant readonly container views") {
			t.Fatalf("wanted explicit readonly covariance refusal, got %v", err)
		}
	}
}

func TestStep4ReadonlyCovarianceDirect(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
 function mapView(x:ReadonlyMap<string,IRMethod>):ReadonlyMap<string,Method|Unsupported>{return x;}
 function setView(x:ReadonlySet<IRMethod>):ReadonlySet<Method|Unsupported>{return x;}
 function arrayView(x:readonly IRMethod[]):readonly (Method|Unsupported)[]{return x;}
 function readMap(x:ReadonlyMap<string,Method|Unsupported>):string{return [...x.values()].map(v=>v.kind).join(",");}
 class Probe {static run():string {const a:IRMethod={kind:"method",name:"a",extra:true};const m=new Map<string,IRMethod>([["a",a]]),s=new Set<IRMethod>([a]),r:IRMethod[]=[a];const vm=mapView(m),vs=setView(s),vr=arrayView(r);m.set("b",a);s.add({kind:"method",name:"b",extra:false});r.push(a);const assigned:ReadonlyMap<string,Method|Unsupported>=m;let kinds="";for(const v of vs)kinds+=v.kind;return (vm===m)+":"+(vs===s)+":"+(vr===r)+":"+kinds+":"+readMap(m)+":"+readMap(vm)+":"+readMap(assigned)+":"+vs.size+":"+[...vs].map(v=>v.kind).join(",")+":"+vr.length+":"+vr[1].kind+":"+vs.has(a);}}
 `
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}

func TestStep4ReadonlyCovarianceObjectKeys(t *testing.T) {
	source := `interface Key{readonly name:string}interface Actual{readonly name:string;readonly extra:boolean}
 function view(x:ReadonlyMap<Key,Actual>):ReadonlyMap<Key,Key>{return x;}
 class Probe {static run():string {const a:Actual={name:"a",extra:true};const foreign:Actual={name:"a",extra:false};const m=new Map<Key,Actual>(),s=new Set<Key>();m.set(a,a);s.add(a);const v=view(m);const stored=[...v.keys()][0];return m.has(foreign)+":"+(m.get(foreign)===undefined)+":"+v.has(foreign)+":"+(v.get(foreign)===undefined)+":"+s.has(foreign)+":"+m.has(a)+":"+v.has(stored)+":"+v.get(a)!.name;}}
 `
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}

func TestStep4ReadonlyCovariancePrimitive(t *testing.T) {
	source := `function view(x:ReadonlySet<string>):ReadonlySet<unknown>{return x;}
 class Probe {static run():string {const s=new Set<string>(["a"]);const v=view(s);s.add("b");return v.size+":"+v.has("a")+":"+[...v].map(x=>String(x)).join(",");}}`
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}

func TestStep4ReadonlyCovariancePrimitiveLiveField(t *testing.T) {
	for _, tc := range []struct {
		name       string
		keyType    string
		literal    string
		literalKey string
	}{
		{name: "string", keyType: "string", literal: `"a"`, literalKey: `"a"`},
		{name: "number", keyType: "number", literal: "7", literalKey: "7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := fmt.Sprintf(`interface Source{readonly tags:ReadonlySet<%s>}
interface Target{readonly tags:ReadonlySet<%s>}
class Factory {static make():Source {const value:%s=%s;return {tags:new Set<%s>([value])}}}`,
				tc.literalKey, tc.keyType, tc.literalKey, tc.literal, tc.literalKey)
			source := prefix + fmt.Sprintf(`function read(holder:Target,value:%s):string {const view=holder.tags;return view.size+":"+view.has(value)}
class Probe {static run():string {return read(Factory.make(),%s)}}`,
				tc.keyType, tc.literal)
			readonlyStrictCheck(t, map[string]string{"probe.ts": source})
			readonlyGoOracle(t, source, false)

			trapSource := prefix + fmt.Sprintf(`function write(x:Set<%s>,value:%s):void{x.add(value);x.delete(value)}
function trap(holder:Target,value:%s):string {const view=holder.tags;let out="";try{write(view as unknown as Set<%s>,value);out+="add"}catch{out+="caught"}try{(view as unknown as Set<%s>).delete(value);out+="delete"}catch{out+="caught"}return out}
class TrapProbe {static run():string {const value:%s=%s;return trap(Factory.make(),value)}}`,
				tc.keyType, tc.keyType, tc.keyType, tc.keyType, tc.keyType,
				tc.literal, tc.literal)
			p := structuralUnionProgram(t, trapSource)
			if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.TrapProbe", "run"))); got != "caughtcaught\n" {
				t.Fatalf("Go writes %q, wanted caughtcaught", got)
			}
		})
	}
}

func TestStep4ReadonlyCovarianceMutableRefused(t *testing.T) {
	for _, target := range []string{"Map<string,Method|Unsupported>", "Set<Method|Unsupported>", "(Method|Unsupported)[]", "Map<string,Method>", "Set<Method>", "Method[]"} {
		t.Run(target, func(t *testing.T) {
			sourceType, init := "Map<string,IRMethod>", "new Map<string,IRMethod>()"
			if strings.HasPrefix(target, "Set") {
				sourceType, init = "Set<IRMethod>", "new Set<IRMethod>()"
			} else if strings.HasSuffix(target, "[]") {
				sourceType, init = "IRMethod[]", "[]"
			}
			source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + fmt.Sprintf("class Probe{static run():void {const source:%s=%s;const target:%s=source;}}", sourceType, init, target)
			readonlyStrictCheck(t, map[string]string{"probe.ts": source})
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600)
			os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"files":["probe.ts"]}`), 0600)
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			prog, diags, err := p.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(hir.Dump(prog), "container.readonlyView") {
				t.Fatal("mutable target received readonly view")
			}
			if len(hir.Verify(prog)) == 0 {
				t.Fatalf("mutable target must retain refusal, lower diagnostics %v", diags)
			}
		})
	}
}

func TestStep4ReadonlyCovarianceLazyArray(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
 class Probe {static run():string {const invalid:any={kind:"invalid"};const rows:IRMethod[]=[invalid];const view:readonly (Method|Unsupported)[]=rows;const size=view.length;rows[0]={kind:"method",name:"fixed",extra:true};return size+":"+view[0].kind;}}`
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}

func TestStep4ReadonlyCovarianceOptional(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
 function mapView(x:ReadonlyMap<string,IRMethod>|undefined):ReadonlyMap<string,Method|Unsupported>|undefined{return x;}
 function setView(x:ReadonlySet<IRMethod>|undefined):ReadonlySet<Method|Unsupported>|undefined{return x;}
 function arrayView(x:readonly IRMethod[]|undefined):readonly (Method|Unsupported)[]|undefined{return x;}
 class Probe {static run():string {const a:IRMethod={kind:"method",name:"a",extra:true};const m=new Map<string,IRMethod>([["a",a]]),s=new Set<IRMethod>([a]),rows:IRMethod[]=[a];return (mapView(undefined)===undefined)+":"+(setView(undefined)===undefined)+":"+(arrayView(undefined)===undefined)+":"+mapView(m)!.get("a")!.kind+":"+setView(s)!.size+":"+arrayView(rows)![0].kind;}}
 `
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}
