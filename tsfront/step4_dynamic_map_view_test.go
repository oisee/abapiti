package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

const dynamicMapOptionalSource = `interface Entry {module:{name:string;body:readonly number[]}}
function read(program:any):string {
  const modules:Map<string,Entry>|undefined=program.functionModules;
  return [...(modules?.values?.()??[])].map((fm):string=>fm.module.name+":"+fm.module.body.length).join(",");
}
class Probe {
  static run():string {
    const source=new Map<string,{module:{name:string;body:number[]};extra:boolean}>();
    source.set("first",{module:{name:"one",body:[1]},extra:true});
    const program:any={functionModules:source};
    const before=read(program);
    source.set("second",{module:{name:"two",body:[2,3]},extra:false});
    return read({})+"|"+read({functionModules:new Map<string,Entry>()})+"|"+before+"|"+read(program);
  }
}
`

func TestStep4DynamicMapOptional(t *testing.T) {
	source := dynamicMapOptionalSource
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}

const dynamicMapNamedSource = `interface Module {name:string}
interface Entry {module:Module}
interface Actual {module:Module;extra:boolean}
function read(program:any):string {const modules:Map<string,Entry>|undefined=program.functionModules;return [...(modules?.values?.()??[])].map((fm):string=>fm.module.name).join(",");}
class Probe {static run():string {const source=new Map<string,Actual>();source.set("one",{module:{name:"one"},extra:true});const program:any={functionModules:source};return read({})+"|"+read(program);}}`

func TestStep4DynamicMapNamed(t *testing.T) {
	p := structuralUnionProgram(t, dynamicMapNamedSource)
	want := conversionOracle(t, dynamicMapNamedSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatal(got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}

const dynamicMapLiveSource = `interface ViewEntry {module:{name:string}}
interface ConcreteEntry {module:{name:string};extra:boolean}
function keep(value:any):Map<string,ViewEntry>{return value;}
class Probe {
  static run():string {
    const raw=new Map<string,ConcreteEntry>();
    const first:ConcreteEntry={module:{name:"before"},extra:true};
    raw.set("a",first);
    const kept=keep(raw);
    const identity=(kept as unknown)===(raw as unknown);
    const before=kept.get("a")!.module.name;
    first.module.name="late";
    const late=kept.get("a")!.module.name;
    raw.set("b",{module:{name:"second"},extra:false});
    raw.delete("a");
    raw.set("a",{module:{name:"last"},extra:true});
    return identity+"|"+before+":"+late+"|"+kept.size+"|"+[...kept.keys()].join(",")+"|"+[...kept.values()].map((e):string=>e.module.name).join(",");
  }
}
`

func dynamicMapOracle(t *testing.T, source string) {
	t.Helper()
	p := structuralUnionProgram(t, source)
	want := conversionOracle(t, source)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}
func TestStep4DynamicMapLive(t *testing.T) { dynamicMapOracle(t, dynamicMapLiveSource) }
func TestStep4DynamicMapIdentityOnly(t *testing.T) {
	dynamicMapOracle(t, `function keep(value:any):Map<string,number>{return value;}class Probe{static run():string{const raw=new Map<string,number>();const first=keep(raw),second=keep(raw);return (first===raw)+":"+(first===second);}}`)
}
func TestStep4DynamicMapLazyValues(t *testing.T) {
	dynamicMapOracle(t, `interface Row{name:string}function keep(value:any):ReadonlyMap<string,Row>{return value;}class Probe{static run():string{const raw=new Map<string,any>();raw.set("bad",17);const kept=keep(raw);const before=kept.size+":"+kept.has("bad")+":"+[...kept.keys()].join(",");raw.set("bad",{name:"fixed"});return before+":"+kept.get("bad")!.name;}}`)
}
func TestStep4DynamicMapWrites(t *testing.T) {
	dynamicMapOracle(t, `interface Row{name:string}interface Actual{name:string;extra:boolean}function keep(value:any):Map<string,Row>{return value;}class Probe{static run():string{const raw=new Map<string,Actual>();const first:Actual={name:"first",extra:true};raw.set("a",first);const kept=keep(raw);const row=kept.get("a")!;row.name="changed";kept.set("b",row);const result=(kept.get("b")===first)+":"+raw.get("b")!.name+":"+kept.delete("a")+":"+[...raw.keys()].join(",");kept.clear();return result+":"+raw.size;}}`)
}
func TestStep4DynamicMapRejectsInvalid(t *testing.T) {
	for _, tc := range []struct{ name, value, use string }{
		{"record", `({x:{name:"fake"}})`, `const n=kept.size;`},
		{"parsed", `JSON.parse('{"x":{"name":"fake"}}')`, `const n=kept.size;`},
		{"number", `17`, `const n=kept.size;`},
		{"null", `null`, `const n=kept.size;`},
		{"undefined", `undefined`, `const n=kept.size;`},
		{"numericKeys", `new Map<number,Row>()`, `const n=kept.size;`},
		{"badValue", `new Map<string,any>([["x",17]])`, `const n=kept.get("x")!.name;`},
		{"badField", `new Map<string,any>([["x",{name:17}]])`, `const n=kept.get("x")!.name;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `interface Row{name:string}function keep(value:any):Map<string,Row>{return value;}class Probe{static run():string{return "ok";}static invalid():void{const raw=` + tc.value + `;const kept=keep(raw);` + tc.use + `}}`
			p := structuralUnionProgram(t, source)
			t.Run("Go", func(t *testing.T) {
				main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("invalid map must refuse with host range fault")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
				if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
					t.Fatal(got)
				}
			})
			t.Run("ABAP", func(t *testing.T) {
				exception := "cx_sy_move_cast_error"
				if tc.name == "badValue" {
					exception = "view.refusal"
				}
				storageABAPProbe(t, p, "probe.ts.Probe", "ok", "invalid", exception)
			})
		})
	}
}

func TestStep4DynamicMapNullable(t *testing.T) {
	dynamicMapOracle(t, `interface Row{name:string}function read(value:Map<string,Row>|null|undefined):string{return [...(value?.values?.()??[])].map((r):string=>r.name).join(",");}class Probe{static run():string{const raw=new Map<string,Row>();raw.set("a",{name:"yes"});return read(undefined)+"|"+read(null)+"|"+read(raw);}}`)
}
func TestStep4DynamicMapNominalValues(t *testing.T) {
	source := `class Secret{private token=1;name="native";read():string{return this.name;}}function keep(raw:any):Map<string,Secret>{return raw;}class Probe{static run():string{const raw=new Map<string,Secret>();const row=new Secret();raw.set("x",row);const kept=keep(raw);return kept.get("x")!.read()+":"+(kept.get("x")===row);}static invalid():void{const raw=new Map<string,any>();raw.set("x",{name:"fake"});const kept=keep(raw);const row=kept.get("x");}}`
	p := structuralUnionProgram(t, source)
	want := conversionOracle(t, source)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatal(got, want)
		}
		main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("nominal Map value must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
		if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
			t.Fatal(got)
		}
	})
	t.Run("ABAP", func(t *testing.T) {
		storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "invalid", "cx_sy_move_cast_error")
	})
}

func TestStep4DynamicMapOptionalValues(t *testing.T) {
	dynamicMapOracle(t, `function keep(raw:any):Map<string,number|undefined>{return raw;}class Probe{static run():string{const raw=new Map<string,number|undefined>();raw.set("missing",undefined).set("number",2);const kept=keep(raw);const before=kept.has("missing")+":"+(kept.get("missing")===undefined)+":"+kept.get("number");kept.set("number",undefined);kept.set("added",3);return before+":"+(raw.get("number")===undefined)+":"+raw.get("added");}}`)
}
func TestStep4DynamicMapBoxedWrites(t *testing.T) {
	dynamicMapOracle(t, `interface Row{name:string}function keep(raw:any):Map<string,Row>{return raw;}class Probe{static run():string{const raw=new Map<string,any>();raw.set("a",{name:"before"});const kept=keep(raw);const row:Row={name:"new"};kept.set("b",row);return raw.get("b").name+":"+(raw.get("b")===row);}}`)
}

func TestStep4DynamicMapArrayValues(t *testing.T) {
	dynamicMapOracle(t, `interface Row{name:string}interface Actual{name:string;extra:boolean}function keep(raw:any):ReadonlyMap<string,readonly Row[]>{return raw;}class Probe{static run():string{const raw=new Map<string,Actual[]>();const rows:Actual[]=[{name:"first",extra:true}];raw.set("x",rows);const kept=keep(raw);const stored=kept.get("x")!;rows[0].name="changed";rows.push({name:"late",extra:false});return (stored===rows)+":"+stored[0].name+":"+stored.length+":"+stored[1].name;}}`)
}
