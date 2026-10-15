package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestRecursiveSubsetsNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"shared-secondary-negative", `type Tree={k:'item';mode:'num';value:number}|{k:'item';mode:'text';value:string}|{k:'branch';children:Tree[]};
class Probe {
static read(x:Tree):string {if(x.k!=='item'){let out='';for(const y of x.children)out=out+Probe.read(y);return out;}if(x.mode==='num')return 'n'+x.value;return 's'+x.value;}
static run():string {const x:Tree={k:'branch',children:[{k:'item',mode:'num',value:3},{k:'item',mode:'text',value:'ok'}]};return Probe.read(x);}}`},
		{"multi-literal-fallthrough", `type Tree={k:'a'|'b';value:number}|{k:'branch';children:Tree[]}|{k:'end';text:string};
class Probe {static read(x:Tree):number {let sum=0;switch(x.k){case 'a':sum=x.value;case 'b':sum=sum+x.value;break;case 'branch':{for(const y of x.children)sum=sum+Probe.read(y);return sum;}default:return x.text.length;}return sum;}
static run():number {const x:Tree={k:'branch',children:[{k:'a',value:3},{k:'b',value:5},{k:'end',text:'ok'}]};return Probe.read(x);}}`},
		{"extract-one-many-indexed", `type Tree={k:'a';value:number}|{k:'b';value:number}|{k:'branch';children:Tree[]};
type One=Extract<Tree,{k:'a'}>;type Many=Extract<Tree,{k:'a'|'b'}>;type None=Extract<Tree,{k:'missing'}>;interface Holder{root:Tree;items:Many[];}type Indexed=Holder['root'];type Value=One['value'];
class Probe {static one(x:One):Value{return x.value;}static many(x:Many):number{return x['value'];}static widen(x:Many):Tree{return x;}static tree(x:Indexed):number{if(x.k==='branch'){let sum=0;for(const y of x.children)sum=sum+Probe.tree(y);return sum;}return Probe.many(x);}
static run():number {const a:One={k:'a',value:2};const b:Many={k:'b',value:7};const h:Holder={root:{k:'branch',children:[a,b]},items:[a,b]};return Probe.one(a)+Probe.many(b)+Probe.tree(h.root)+Probe.tree(Probe.widen(b))+h.items.length;}}`},
		{"extract-none", `type Tree={k:'a';value:number}|{k:'branch';children:Tree[]};class Probe {static impossible():Extract<Tree,{k:'none'}>{throw new Error('none');}static run():number{try{Probe.impossible();return 0;}catch(e){return 1;}}}`},
		{"utility-first", `type Tree={k:'a';value:number}|{k:'b';value:number}|{k:'branch';children:Tree[]};
class Probe {static one(x:Extract<Tree,{k:'a'}>):number{return x.value;}static many(x:Extract<Tree,{k:'a'|'b'}>):number{return x.value;}static run():number{return Probe.one({k:'a',value:4})+Probe.many({k:'b',value:6});}}`},
		{"shared-typeof", `type Tree={k:'item';value:number;children:Tree[]}|{k:'item';value:string;children:Tree[]};class Probe{static read(x:Tree):string{if(typeof x.value==='number')return 'n'+x.value;return 's'+x.value;}static run():string{const a:Tree={k:'item',value:2,children:[]};const b:Tree={k:'item',value:'ok',children:[]};return Probe.read(a)+Probe.read(b);}}`},
		{"typed-expr-intersection", `type Type={k:'num'}|{k:'array';item:Type};type Expr={e:'num';value:number}|{e:'pair';children:Expr[]};type TypedExpr=Expr&{type:Type};
class Probe {static base(x:Expr):Expr{return x;}static read(x:TypedExpr):number{let n=0;if(x.type.k==='array')n=1;if(x.e==='num')return n+x.value;for(const y of x.children){if(y.e==='num')n=n+y.value;}return n;}
static run():number{const x:TypedExpr={e:'pair',type:{k:'array',item:{k:'num'}},children:[{e:'num',value:4},{e:'num',value:6}]};return Probe.read(x);}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, tc.source).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": tc.source}, []string{"probe.ts"})
			if t.Failed() {
				return
			}
			if strings.Contains(hir.Dump(p), "trap ") {
				t.Fatal("trapped body")
			}
			if _, err := emitABAP(t, p); err == nil {
				t.Fatal("ABAP accepted union ABI")
			}
			if files, _, err := emitABAPTestNamed(t, p, map[string]bool{}); err == nil || len(files) != 0 {
				t.Fatal("ABAP test emission did not refuse before body traps", err)
			}
			suffix := ""
			if strings.Contains(tc.source, "run():string") {
				suffix = ".String()"
			}
			got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s()%s)", goEntry("probe.ts.Probe", "run"), suffix))
			if got != string(want) {
				t.Fatalf("Go %q Node %q", got, want)
			}
			encoded, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			var restored hir.Program
			if err = json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if errors := hir.Verify(&restored); len(errors) > 0 {
				t.Fatal(errors)
			}
			if hir.Dump(p) != hir.Dump(&restored) {
				t.Fatal("lost subset metadata")
			}
		})
	}
}

func TestRecursiveSubsetsInvalidPayloadRefusal(t *testing.T) {
	const types = `type Tree={k:'item';mode:'num';value:number}|{k:'item';mode:'text';value:string}|{k:'branch';children:Tree[]};`
	for _, tc := range []struct{ name, annotation, payload string }{
		{"shared-number", "Tree", `{k:'item',mode:'num',value:'wrong'}`},
		{"shared-string", "Tree", `{k:'item',mode:'text',value:42}`},
		{"secondary", "Tree", `{k:'item',mode:'unknown',value:42}`},
		{"extract-one", "Extract<Tree,{mode:'num'}>", `{k:'item',mode:'text',value:'wrong'}`},
		{"extract-many", "Extract<Tree,{k:'item'}>", `{k:'branch',children:[]}`},
		{"extract-none", "Extract<Tree,{k:'none'}>", `{k:'item',mode:'num',value:1}`},
		{"wrong-array", "Tree", `{k:'branch',children:42}`},
		{"missing-field", "Tree", `{k:'item',mode:'num'}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The independent Node predicate observes why this is not a member of the
			// requested subset, even though JavaScript permits constructing the object.
			predicate := `(x.k==='item'&&((x.mode==='num'&&typeof x.value==='number')||(x.mode==='text'&&typeof x.value==='string')))||(x.k==='branch'&&Array.isArray(x.children))`
			if tc.name == "extract-one" {
				predicate = `x.k==='item'&&x.mode==='num'&&typeof x.value==='number'`
			}
			if tc.name == "extract-many" {
				predicate = `x.k==='item'`
			}
			if tc.name == "extract-none" {
				predicate = `false`
			}
			want, err := exec.Command("node", "-e", `const x=`+tc.payload+`;console.log(Boolean(`+predicate+`));`).CombinedOutput()
			if err != nil || string(want) != "false\n" {
				t.Fatal("invalid oracle", err, string(want))
			}
			source := types + `class Probe{static bad():` + tc.annotation + `{const x:` + tc.annotation + `=` + tc.payload + `;return x;}}`
			dir := t.TempDir()
			for name, body := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			project, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			p, diags, err := project.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			refused := false
			for _, d := range diags {
				if !strings.HasPrefix(d.Category, "note-") {
					refused = true
				}
			}
			if !refused && len(hir.Verify(p)) == 0 {
				t.Fatal("invalid payload silently admitted")
			}
		})
	}
}

func TestRecursiveSubsetsLivePayloadRefusalNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, mutation string }{
		{"scalar", `erased.value='wrong';`},
		{"secondary", `erased.mode='text';`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `type Tree={k:'item';mode:'num';value:number}|{k:'item';mode:'text';value:string}|{k:'branch';children:Tree[]};class Probe{static read(x:Tree):number{if(x.k==='item'&&x.mode==='num'){const retained=x;const erased:any=x;` + tc.mutation + `return retained.value;}return 0;}static run():number{const x:Tree={k:'item',mode:'num',value:3};return Probe.read(x);}}`
			// The original program sees a payload that violates the retained num proof.
			node := `const x={k:'item',mode:'num',value:3};const erased=x;` + tc.mutation + `console.log(x.mode==='num'&&typeof x.value==='number');`
			want, err := exec.Command("node", "-e", node).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			if t.Failed() {
				return
			}
			got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if r:=recover();r!=nil{if _,ok:=r.(rangeFault);!ok{panic(r)};fmt.Println(false)}}();%s();fmt.Println(true)`, goEntry("probe.ts.Probe", "run")))
			if got != string(want) {
				t.Fatalf("Go proof validity %q Node %q", got, want)
			}
		})
	}
}
