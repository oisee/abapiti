package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStep4NullableRefJoin(t *testing.T) {
	for _, js := range []bool{false, true} {
		t.Run(fmt.Sprint("js=", js), func(t *testing.T) {
			source := `interface Class {readonly super?:string|null;readonly own:boolean}
 let CLASSES=new Map<string,Class>();
 function ownCctor(c:Class):boolean{return c.own;}
 function chainCctor(cls:Class):boolean {
 for(let c:Class|null|undefined=cls;c;c=c.super?CLASSES.get(c.super):null) if(ownCctor(c)) return true;
 return false;
 }
 class Probe {static run():string {
 CLASSES.set("hit",{own:true});
 return chainCctor({super:"hit",own:false})+":"+chainCctor({super:"miss",own:false})+":"+chainCctor({super:null,own:false})+":"+chainCctor({own:false})+":"+chainCctor(JSON.parse("null"))+":"+chainCctor(JSON.parse("{}").missing);
 } static invalid():boolean {const raw=JSON.parse('42');return chainCctor(raw);}}
 `
			file := "probe.ts"
			sources := map[string]string{}
			if js {
				file = "probe.js"
				source = `/** @type {Map<string, import("./emit-go-types.ts").Class>} */ let CLASSES=new Map();
 /** @param {import("./emit-go-types.ts").Class} c @returns {boolean} */ function ownCctor(c){return c.own;}
 /** @param {import("./emit-go-types.ts").Class} cls @returns {boolean} */ function chainCctor(cls){
 for(let c=cls;c;c=c.super?CLASSES.get(c.super):null) if(ownCctor(c)) return true;
 return false;
 }
 class Probe {static run(){
 CLASSES.set("hit",{own:true});
 return chainCctor({super:"hit",own:false})+":"+chainCctor({super:"miss",own:false})+":"+chainCctor({super:null,own:false})+":"+chainCctor({own:false})+":"+chainCctor(JSON.parse("null"))+":"+chainCctor(JSON.parse("{}").missing);
 } static invalid(){const raw=JSON.parse('42');return chainCctor(raw);}}`
				sources["emit-go-types.ts"] = `export interface Class {readonly super?:string|null;readonly own:boolean}`
			}
			sources[file] = source
			sources["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"ES2022","allowJs":true,"checkJs":true,"noEmit":true},"files":["` + file + `"]}`
			dir := t.TempDir()
			for name, s := range sources {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0600); err != nil {
					t.Fatal(err)
				}
			}
			project, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			p, ds, err := project.LowerWithOptions([]string{file}, LowerOptions{})
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(p); len(errs) > 0 {
				t.Fatal(errs)
			}
			for _, c := range p.Classes {
				for _, m := range c.Methods {
					if js && m.Name == "chainCctor" {
						viewWalk(m.Body, nil, func(x, _ *hir.Expr) {
							if x.Kind == hir.Conditional && x.Type.Kind == hir.Dynamic {
								t.Error("nullable reference loop joined through dynamic")
							}
						})
					}
				}
			}
			want := "true:false:false:false:false:false"
			oracle, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
			if err != nil || string(oracle) != want+"\n" {
				t.Fatalf("Node: %v %s", err, oracle)
			}
			if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry(file+".Probe", "run"))); got != want+"\n" {
				t.Fatal(got)
			}
			main := fmt.Sprintf(`defer func(){if x:=recover();x!=nil {if _,ok:=x.(rangeFault);!ok {panic(x)}} else {panic("primitive admitted")};fmt.Println("refused")}();%s()`, goEntry(file+".Probe", "invalid"))
			if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
			storageABAPProbe(t, p, file+".Probe", want, "invalid")
		})
	}
}

func TestStep4NullableRefJoinOperators(t *testing.T) {
	for _, expression := range []string{"flag ? table.get(key) : null", "flag ? null : table.get(key)", "flag ? table.get(key) : undefined", "table.get(key) ?? null", "table.get(key) ?? undefined", "table.get(key) || null", "table.get(key) || undefined"} {
		t.Run(expression, func(t *testing.T) {
			source := `interface Item {readonly value:string}
const table=new Map<string,Item>();
function pick(flag:boolean,key:string):Item|undefined {return (` + expression + `) as Item|undefined;}
class Probe {static run():string {const item={value:"live"};table.set("hit",item);let out="";
for(const flag of [true,false]) for(const key of ["hit","miss"]) {const value=pick(flag,key);out+=(value?value.value:"absent")+":"+(value===item)+";";}return out;}}`
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			for _, c := range p.Classes {
				for _, m := range c.Methods {
					if m.Name == "pick" {
						viewWalk(m.Body, nil, func(x, _ *hir.Expr) {
							if x.Kind == hir.Conditional && (x.Type.Kind == hir.Dynamic || x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic) {
								t.Error("reference join uses tagged storage")
							}
						})
					}
				}
			}
			conditionalMatchesNode(t, source, p, "Probe")
			if _, err := emitABAP(t, p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStep4NullableRefJoinTaggedNull(t *testing.T) {
	source := `interface Item {readonly value:string}
 function conditional(flag:boolean,item:Item):any {return flag?item:null;}
 function fallback(item:Item|undefined):any {return item??null;}
 function logical(item:Item|undefined):any {return item||null;}
 class Probe {static run():string {const item={value:"live"};return (conditional(false,item)===null)+":"+(fallback(undefined)===null)+":"+(logical(undefined)===null)+":"+(conditional(true,item)===item);}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	conditionalMatchesNode(t, source, p, "Probe")
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
