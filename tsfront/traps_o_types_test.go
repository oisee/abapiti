package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuralPrimitivePresenceWideningNodeOracle(t *testing.T) {
	source := `interface Narrow {uri:string;version:number;}
interface Wide {uri:string;version:number|null;}
class Probe {
 static view(x:Wide):Wide {x.version=2;return x;}
 static bad(x:Wide):void {x.version=null;}
 static run():string {const x:Narrow={uri:"file",version:1};const y=Probe.view(x);x.version=3;return (x===y)+":"+y.version+":"+JSON.stringify(y);}
 static refused():void {const x:Narrow={uri:"file",version:1};Probe.bad(x);}
}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	got = runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("required primitive backing accepted null")};fmt.Print("refused")}();%s()`, goEntry("probe.ts.Probe", "refused")))
	if got != "refused" {
		t.Fatal(got)
	}
}

func TestImportedEnumNamespaceNodeOracle(t *testing.T) {
	source := `import * as L from "vscode-languageserver-types";
 export class Probe {static run():string {let out="";for(const key in L.SemanticTokenTypes){out+=key+":";}return out+L.SemanticTokenTypes.keyword;}}`
	p, ds := lspProbe(t, source)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	got := runGoHIR(t, p, "fmt.Print("+goEntry("main.ts.Probe", "run")+"().String())")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(home, "dev/abaplint/packages/core/node_modules/vscode-languageserver-types")
	out, err := exec.Command("node", "-e", `const L=require(process.argv[1]);if(require(process.argv[1]+"/package.json").version!=="3.18.0")throw Error("wrong LSP pin");let out="";for(const key in L.SemanticTokenTypes){out+=key+":";}process.stdout.write(out+L.SemanticTokenTypes.keyword);`, pkg).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != string(out) {
		t.Fatalf("Go %q Node %q", got, out)
	}
}

func TestUnionConstructorAndEnumFlowNodeOracle(t *testing.T) {
	source := `interface Marker {run():string;}
class A {run():string{return "A";}}
class B {}
class Token {}
class Derived extends A {}
enum Tokens { keyword="keyword", string="string", number="number" }
export class Probe {
 static name(x:A|B):string {return x.constructor.name;}
 static mixed(x:Marker|Token):string {return x.constructor.name;}
 static run():string {
  let out=Probe.name(new A())+":"+Probe.name(new B())+":"+Probe.name(new Derived())+":"+Probe.mixed(new A())+":"+Probe.mixed(new Token());
  for(const key in Tokens) {out+=":"+key;}
  return out;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	path := filepath.Join(t.TempDir(), "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	typescript := filepath.Join(home, "dev/abaplint/node_modules/typescript")
	out, err := exec.Command("node", "-e", `const ts=require(process.argv[1]);const src=require("node:fs").readFileSync(process.argv[2],"utf8");const js=ts.transpileModule(src,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText;new Function("exports",js)({});`, typescript, path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSpace(string(out)) {
		t.Fatalf("Go %q Node %q", got, out)
	}
}
