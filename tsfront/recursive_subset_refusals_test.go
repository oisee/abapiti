package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Each invalid program still has a defined JavaScript observation. Original
// Node independently demonstrates that its result violates the promised field
// or subset contract; the compiler must refuse before executable emission.
func TestRecursiveSubsetFeatureRefusalsNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, source, valid string }{
		{"multi-literal-payload", `type Tree={k:'a'|'b';value:number}|{k:'branch';children:Tree[]};class Probe{static bad():Tree{const x:Tree={k:'a',value:'wrong'};return x;}}`, `typeof Probe.bad().value==='number'`},
		{"multi-literal-tag", `type Tree={k:'a'|'b';value:number}|{k:'branch';children:Tree[]};class Probe{static bad():Tree{const x:Tree={k:'other',value:3};return x;}}`, `['a','b','branch'].includes(Probe.bad().k)`},
		{"negative-guard", `type Tree={k:'item';value:number}|{k:'branch';children:Tree[]};class Probe{static bad(x:Tree):number{if(x.k!=='item')return x.value;return 0;}}`, `typeof Probe.bad({k:'branch',children:[]})==='number'`},
		{"fallthrough-payload", `type Tree={k:'a';a:number}|{k:'b';value:number}|{k:'branch';children:Tree[]};class Probe{static bad(x:Tree):number{let n=0;switch(x.k){case 'a':n=x.a;case 'b':return x.value;default:return n;}}}`, `typeof Probe.bad({k:'a',a:1})==='number'`},
		{"indexed-field", `type Tree={k:'item';value:number}|{k:'branch';children:Tree[]};type Count=Extract<Tree,{k:'item'}>['value'];class Probe{static bad():Count{return 'wrong';}}`, `typeof Probe.bad()==='number'`},
		{"typed-expr-required-field", `type Type={k:'num'}|{k:'array';item:Type};type Expr={e:'num';value:number}|{e:'pair';children:Expr[]};type TypedExpr=Expr&{type:Type};class Probe{static base(x:Expr):Expr{return x;}static bad():TypedExpr{const x:TypedExpr={e:'num',value:3};return x;}}`, `Probe.bad().type!==undefined`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log("+process.argv[2]+")")`, tc.source, tc.valid).CombinedOutput()
			if err != nil || string(want) != "false\n" {
				t.Fatalf("Node contract observation: %v %s", err, want)
			}
			dir := t.TempDir()
			for name, body := range map[string]string{"probe.ts": tc.source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			project, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			p, ds, err := project.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range ds {
				if !strings.HasPrefix(d.Category, "note-") {
					return
				}
			}
			if len(hir.Verify(p)) > 0 {
				return
			}
			t.Fatal("invalid payload/flow silently accepted")
		})
	}
}
