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
	"github.com/oisee/abapiti/hir/rewrite"
)

func TestRecursiveFamilyTreeNodeOracle(t *testing.T) {
	source := `type Tree = {k:'leaf';value:number} | {k:'branch';left:Tree;right:Tree};
class Probe {
 static total(x:Tree):number {if(x.k==='leaf') return x.value; return Probe.total(x.left)+Probe.total(x.right);}
 static run():number {const t:Tree={k:'branch',left:{k:'leaf',value:2},right:{k:'branch',left:{k:'leaf',value:3},right:{k:'leaf',value:5}}};return Probe.total(t);}
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if len(p.Unions) != 1 || !p.Unions[0].Complete {
		t.Fatal("missing completed family")
	}
	abapUnitProbe(t, cloneProgram(t, p), "probe.ts.Probe", strings.TrimSpace(string(want)), "f", "")
	if strings.Contains(hir.Dump(p), "trap ") {
		t.Fatal("recursive body was trapped")
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	facts := rewrite.Extract(p)
	if len(facts.Facts("union")) != len(p.Unions) || len(facts.Facts("union_alternative")) != 2 {
		t.Fatal("rewrite facts dropped family membership")
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var restored hir.Program
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	hir.Inline(&restored)
	if errors := hir.Verify(&restored); len(errors) != 0 {
		t.Fatal(errors)
	}
	if again := runGoHIR(t, &restored, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); again != string(want) {
		t.Fatal("serialized/inlined recursion changed execution", again)
	}
}

func TestRecursiveFamilyMutualNodeOracle(t *testing.T) {
	declarations := []string{
		`type Meta = {line:number;label:string;};`,
		`export type Tree = Meta & ({k:'leaf';value:number}|{k:'branch';children:Tree[][];links:Map<string,Expr>});`,
		`export type Expr = Meta & ({e:'num';value:number}|{e:'tree';tree:Tree}|{e:'seq';statement:Stmt});`,
		`export type Stmt = Meta & ({s:'expr';expr:Expr}|{s:'block';body:Stmt[]});`,
	}
	probe := `import type {Tree,Expr,Stmt} from './types.ts';
type ImportedTree = Tree;
export class Probe {
 static mark=0;
 static effect(n:number):number {Probe.mark=Probe.mark*10+n;return n;}
 static tree(t:ImportedTree):number {let sum=t.line+t.label.length;if(t.k==='leaf')return sum+t.value;for(const row of t.children){for(const child of row){sum=sum+Probe.tree(child);}}for(const e of t.links.values()){sum=sum+Probe.expr(e);}return sum;}
 static expr(e:Expr):number {const meta=e.line+e.label.length;if(e.e==='num')return meta+e.value;if(e.e==='tree')return meta+Probe.tree(e.tree);return meta+Probe.stmt(e.statement);}
 static stmt(s:Stmt):number {let sum=s.line+s.label.length;if(s.s==='expr')return sum+Probe.expr(s.expr);for(const child of s.body){sum=sum+Probe.stmt(child);}return sum;}
 static run():string {
 const leaf:ImportedTree={value:Probe.effect(1),label:'leaf',k:'leaf',line:Probe.effect(2)};
 const item:Expr={e:'tree',tree:leaf,line:3,label:'expr'};
 const block:Stmt={s:'block',line:4,label:'block',body:[{s:'expr',expr:item,line:5,label:'stmt'}]};
 const links=new Map<string,Expr>();links.set('seq',{e:'seq',statement:block,line:6,label:'seq'});
 const tree:ImportedTree={k:'branch',line:7,label:'root',children:[[leaf],[leaf]],links:links};
 return Probe.tree(tree)+':'+Probe.mark+':'+(tree.children[0][0]===leaf)+':'+Object.keys(leaf).join(',');
 }}
`
	sources := map[string]string{"types.ts": strings.Join(declarations, "\n"), "probe.ts": probe}
	dir := t.TempDir()
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.ts"))+`;console.log(Probe.run());`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, sources, []string{"types.ts", "probe.ts"})
	families := 0
	for _, u := range p.Unions {
		if !strings.HasPrefix(u.Name, "family.subset.") {
			families++
		}
	}
	if families != 3 {
		t.Fatalf("families: %d", len(p.Unions))
	}
	for _, u := range p.Unions {
		if !u.Complete {
			t.Fatal("partial family", u.Name)
		}
		for _, a := range u.Alternatives {
			c := shapeClassOf(p.Classes, a.Payload)
			fields := map[string]bool{}
			for _, f := range c.Fields {
				fields[f.Name] = true
			}
			if !fields["line"] || !fields["label"] || !fields[u.Discriminant] {
				t.Fatal("dropped metadata/tag", u.Name, a.ID)
			}
			wantFields := map[string][]string{"leaf": {"value"}, "branch": {"children", "links"}, "num": {"value"}, "tree": {"tree"}, "seq": {"statement"}, "expr": {"expr"}, "block": {"body"}}
			for _, tag := range a.Tags {
				for _, name := range wantFields[tag] {
					if !fields[name] {
						t.Fatal("dropped required payload field", tag, name)
					}
				}
			}
		}
	}
	if strings.Contains(hir.Dump(p), "trap ") {
		t.Fatal("trapped recursive body")
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, cloneProgram(t, p), "probe.ts.Probe", strings.TrimSpace(string(want)), "")
	permuted := append([]string(nil), declarations...)
	for i, j := 0, len(permuted)-1; i < j; i, j = i+1, j-1 {
		permuted[i], permuted[j] = permuted[j], permuted[i]
	}
	other := map[string]string{"types.ts": strings.Join(permuted, "\n"), "probe.ts": probe}
	other["types.ts"] = strings.ReplaceAll(other["types.ts"], "line:number;label:string;", "label:string;line:number;")
	other["types.ts"] = strings.ReplaceAll(other["types.ts"], "{k:'leaf';value:number}|{k:'branch';children:Tree[][];links:Map<string,Expr>}", "{k:'branch';links:Map<string,Expr>;children:Tree[][]}|{k:'leaf';value:number}")
	q := lowerStatementsProbe(t, other, []string{"probe.ts", "types.ts"})
	if hir.Dump(p) != hir.Dump(q) {
		t.Fatal("HIR dump changed under declaration/input order permutation")
	}
	if again := runGoHIR(t, q, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run"))); again != string(want) {
		t.Fatalf("permuted Go %q Node %q", again, want)
	}
}

func TestRecursiveFamilyAtomicRefusal(t *testing.T) {
	source := `type Bad={k:'leaf';value:number}|{k:'branch';next:Bad;required:(x:number)=>number};
interface Holder {root:Bad;again:Bad;}
class Probe {static read(x:Bad):number {if(x.k==='leaf')return x.value;return Probe.read(x.next);}}`
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
	if len(p.Unions) != 0 {
		t.Fatal("cached a partial family")
	}
	for _, c := range p.Classes {
		if strings.HasPrefix(c.Name, "family.") {
			t.Fatal("published partial payload", c.Name)
		}
	}
	refused := false
	for _, d := range diags {
		refused = refused || !strings.HasPrefix(d.Category, "note-")
	}
	if !refused {
		t.Fatal("unsupported required callback silently accepted")
	}
}

func TestRecursiveFamilyAnonymousLayoutsNodeOracle(t *testing.T) {
	source := `type Tree={k:'leaf';value:number}|{k:'branch';left:{label:string;child:Tree};right:{note:number;node:Tree}};
class Probe {static total(t:Tree):number{if(t.k==='leaf')return t.value;return t.left.label.length+Probe.total(t.left.child)+t.right.note+Probe.total(t.right.node);}
static run():number{const leaf:Tree={k:'leaf',value:3};const tree:Tree={k:'branch',left:{label:'left',child:leaf},right:{note:5,node:leaf}};return Probe.total(tree);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestRecursiveFamilyGenericIdentitiesNodeOracle(t *testing.T) {
	source := `type Tree<T>={tag:'leaf';value:T}|{tag:'branch';left:Tree<T>;right:Tree<T>};
class Probe {static number(t:Tree<number>):number{if(t.tag==='leaf')return t.value;return Probe.number(t.left)+Probe.number(t.right);}
static text(t:Tree<string>):string{if(t.tag==='leaf')return t.value;return Probe.text(t.left)+Probe.text(t.right);}
static run():string{const n:Tree<number>={tag:'branch',left:{tag:'leaf',value:2},right:{tag:'leaf',value:3}};const s:Tree<string>={tag:'branch',left:{tag:'leaf',value:'a'},right:{tag:'leaf',value:'b'}};return Probe.number(n)+':'+Probe.text(s);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if len(p.Unions) != 2 || p.Unions[0].Name == p.Unions[1].Name {
		t.Fatal("resolved type arguments shared a family cache entry")
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestRecursiveFamilyWrongTagRefusal(t *testing.T) {
	for _, tag := range []string{"wrong", "branch"} {
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cached=%t", tag, cached), func(t *testing.T) {
				source := `type Tree={k:'leaf';value:number}|{k:'branch';left:Tree;right:Tree};
class Probe {static total(t:Tree):number{if(t.k==='leaf')return t.value;return Probe.total(t.left)+Probe.total(t.right);}
static run():number{const leaf:Tree={k:'leaf',value:7};const erased:any=leaf;erased.k='wrong';return Probe.total(leaf);}}`
				source = strings.Replace(source, "erased.k='wrong'", "erased.k='"+tag+"'", 1)
				if cached {
					source = strings.Replace(source, "const erased:any=leaf;", "const cached=leaf;const erased:any=leaf;", 1)
					source = strings.Replace(source, "return Probe.total(leaf);", "return cached.value;", 1)
				}
				p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
				got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if r:=recover();r!=nil{if _,ok:=r.(rangeFault);!ok{panic(r)};fmt.Println("rejected")}}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "run")))
				if got != "rejected\n" {
					t.Fatalf("wrong tag was accepted: %q", got)
				}
			})
		}
	}
}

func TestRecursiveFamilyPayloadRefusals(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"wrong-literal", `const x:Tree={k:'other',value:1};return x;`, "no admitted tag"},
		{"missing-required", `const x:Tree={k:'branch',left:{k:'leaf',value:1}};return x;`, "misses field right"},
		{"unnarrowed-write", `x.k='leaf';return x;`, "write requires narrowing"},
		{"narrowed-write", `if(x.k==='leaf'){x.value=2;}return x;`, "alias-invalidation policy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := `type Tree={k:'leaf';value:number}|{k:'branch';left:Tree;right:Tree}; class Probe{static bad(`
			if strings.Contains(tc.name, "write") {
				source += `x:Tree`
			} else {
				source += `unused:number`
			}
			source += `):Tree{` + tc.body + `}}`
			for name, body := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			project, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, diags, err := project.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diags {
				if !strings.HasPrefix(d.Category, "note-") && strings.Contains(d.Message, tc.want) {
					return
				}
			}
			t.Fatalf("missing explicit refusal %q: %v", tc.want, diags)
		})
	}
}
