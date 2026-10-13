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

func TestUnionFieldViewNodeOracle(t *testing.T) {
	source := `interface Nested {text:string;} interface Left {value:number; nested:Nested; extra:string; maybe?:string;}
 interface Right {value:number; nested:Nested; other:boolean; maybe?:string;}
 class Probe {
 static calls=0;
 static receive(x:Left|Right):Left|Right {Probe.calls=Probe.calls+1;return x;}
 static read(x:Left|Right):string {return Probe.receive(x).value+":"+x.nested.text+":"+(x.maybe===undefined);}
 static writeNested(x:Left|Right):void {x.nested.text="through";}
 static same(x:Left|Right, n:Nested):boolean {return x.nested===n;}
 static run():string {const a:Left={value:1,nested:{text:"a"},extra:"kept"};const b:Right={value:2,nested:{text:"b"},other:true,maybe:"yes"};const first=Probe.read(a);Probe.writeNested(a);const seen=a.nested.text;const same=Probe.same(a,a.nested);a.value=3;a.nested.text="live";return first+"|"+Probe.read(a)+"|"+Probe.read(b)+"|"+Probe.calls+"|"+a.extra+"|"+seen+"|"+same;}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if !strings.Contains(hir.Dump(p), "union.fields.") {
		t.Fatal("missing shared field view")
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestUnionFieldCompatibleNodeOracle(t *testing.T) {
	source := `interface Small {text:string;} interface Large {text:string;extra:number;}
 interface A {nested:Small;value?:number; a:string;} interface B {nested:Large;value:number; b:boolean;}
 class Probe {static read(x:A|B):string{return x.nested.text+":"+(x.value===undefined);}
 static run():string {const a:A={nested:{text:"small"},a:"a"};const b:B={nested:{text:"large",extra:2},value:0,b:true};return Probe.read(a)+"|"+Probe.read(b);}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestUnionFieldNativeNodeOracle(t *testing.T) {
	source := `class A {value=1; extra="a";} class B {value=2; extra=true;}
 class Probe {static calls=0;static read(x:A|B):number {Probe.calls=Probe.calls+1;return x.value;}
 static run():string {const a=new A();const b=new B();const first=Probe.read(a);a.value=3;return first+":"+Probe.read(a)+":"+Probe.read(b)+":"+Probe.calls;}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestUnionFieldNarrowedNodeOracle(t *testing.T) {
	source := `class A {value=1;} class B {other=2;}
 class Probe {static read(x:A|B):number {if (x instanceof A) {return x.value;} return x.other;}
 static run():string {return Probe.read(new A())+":"+Probe.read(new B());}}`
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

func TestUnionFieldRequiresNarrowing(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"missing", `interface A {value:number;} interface B {other:string;} class Probe {static read(x:A|B):number{return x.value;}}`, "not present in every member"},
		{"write", `interface A {value:number; a:string;} interface B {value:number; b:boolean;} class Probe {static write(x:A|B):void{x.value=2;}}`, "write requires narrowing"},
		{"incompatible", `interface A {value:number;} interface B {value:string;} class Probe {static read(x:A|B):number|string{return x.value;}}`, "incompatible member types"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, diags, err := p.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diags {
				if strings.Contains(d.Message, tc.want) {
					return
				}
			}
			t.Fatalf("missing explicit refusal %q: %v", tc.want, diags)
		})
	}
}
