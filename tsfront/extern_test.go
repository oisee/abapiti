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
	gohir "github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func externProject(t *testing.T, source, host string) (*RegistryLowering, []ExternModule, string) {
	t.Helper()
	dir := t.TempDir()
	for n, s := range map[string]string{"tsconfig.json": `{"compilerOptions":{"target":"es2022","module":"commonjs","strict":true},"files":["probe.ts","host.ts"]}`, "probe.ts": source, "host.ts": host} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := filepath.Abs("../extern/hostio")
	if err != nil {
		t.Fatal(err)
	}
	config := struct {
		Modules []ExternModule `json:"modules"`
	}{[]ExternModule{{File: "host.ts", GoImport: "github.com/oisee/abapiti/extern/hostio", GoDir: root}}}
	raw, _ := json.Marshal(config)
	file := filepath.Join(dir, "externs.json")
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	modules, err := ReadExterns(file)
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := overrides.New()
	r, err := LowerProjectWithOptions(filepath.Join(dir, "tsconfig.json"), []string{"probe.ts", "host.ts"}, registry, nil, LowerOptions{Externs: modules})
	if err != nil {
		t.Fatal(err)
	}
	return r, modules, dir
}
func requireExternClean(t *testing.T, r *RegistryLowering) {
	t.Helper()
	if len(r.Blocking) > 0 || len(r.Verification) > 0 {
		t.Fatalf("blocking: %v\nverification: %v\n%s", r.Blocking, r.Verification, hir.Dump(r.Prog))
	}
	for _, c := range r.Prog.Classes {
		if strings.HasPrefix(c.Name, "host.ts.") {
			t.Fatal("lowered extern body", c.Name)
		}
	}
}
func TestExternHostNodeOracle(t *testing.T) {
	source, err := os.ReadFile("testdata/extern/probe.ts")
	if err != nil {
		t.Fatal(err)
	}
	host, err := os.ReadFile("testdata/extern/host.ts")
	if err != nil {
		t.Fatal(err)
	}
	r, modules, dir := externProject(t, string(source), string(host))
	requireExternClean(t, r)
	files, err := gohir.Emit(r.Prog)
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = ExternGoMod("probe", modules, dir)
	files["main.go"] = fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Println(%s())}\n", goEntry("probe.ts.Probe", "run"))
	build := t.TempDir()
	for n, s := range files {
		if err = os.WriteFile(filepath.Join(build, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(build, "probe")
	cmd := exec.Command("nice", "go", "build", "-o", bin, ".")
	cmd.Dir = build
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s\n%s", err, b, files["extern.go"])
	}
	work := t.TempDir()
	if err = os.Mkdir(filepath.Join(work, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABAPITI_EXTERN_VALUE", "yes")
	cmd = exec.Command("nice", bin)
	cmd.Dir = work
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, got)
	}
	want := "æ中😀|æ中😀|a|//|b/c/|a,z.txt|a:true|z.txt:false|true|true|yes|caught:true\n"
	if string(got) != want {
		t.Fatalf("Go %q want %q", got, want)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Log("Node absent; Go half passed")
		return
	}
	nodeWork := t.TempDir()
	if err = os.Mkdir(filepath.Join(nodeWork, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	nodeHost, err := filepath.Abs("testdata/extern/host.cjs")
	if err != nil {
		t.Fatal(err)
	}
	js := strings.Replace(string(source), `import {readText as read, writeText, listDir, listEntries, basename, dirname, join, gitHead, env} from "./host";`, `const {readText:read, writeText, listDir, listEntries, basename, dirname, join, gitHead, env}=require(`+fmt.Sprintf("%q", nodeHost)+`);`, 1)
	js = strings.Replace(js, `import * as io from "./host";`, `const io=require(`+fmt.Sprintf("%q", nodeHost)+`);`, 1)
	cmd = exec.Command("nice", "node", "--disable-warning=ExperimentalWarning", "-e", `eval(require('node:module').stripTypeScriptTypes(process.argv[1])+'\nconsole.log(Probe.run())')`, js)
	cmd.Dir = nodeWork
	node, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v %s", err, node)
	}
	if string(got) != string(node) {
		t.Fatalf("Go %q Node %q", got, node)
	}
}
func TestExternDiagnostics(t *testing.T) {
	for _, c := range []struct{ name, source, host, want string }{
		{"signature", `import {bad} from './host';class Probe {static run():void {bad(1);}}`, `export function bad(x: Date): string {return ""}`, "unsupported-extern-signature"},
		{"value", `import {f} from './host';class Probe {static run():void {const x=f;}}`, `export function f(): string {return ""}`, "unsupported-extern-use"},
		{"namespace", `import * as io from './host';class Probe {static run():void {const x=io;}}`, `export function f():string{return "";}`, "unsupported-extern-use"},
		{"unused-signature", `class Probe {static run():void {}}`, `export function bad():Date{return new Date();}`, "unsupported-extern-signature"},
		{"variable", `import {x} from './host';class Probe {static run():string {return x;}}`, `export const x:string="no";`, "unsupported-extern-use"},
		{"class", `import {X} from './host';class Probe {static run():void {new X();}}`, `export class X {}`, "unsupported"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, _, _ := externProject(t, c.source, c.host)
			found := false
			for _, d := range r.Blocking {
				if strings.Contains(d.Category, c.want) && strings.Contains(d.Loc, ".ts:") {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing located %s: %v", c.want, r.Blocking)
			}
		})
	}
}
func TestExternABAPRefusal(t *testing.T) {
	r, _, _ := externProject(t, `import {basename} from './host';class Probe {static run():string {return basename("a");}}`, `export function basename(x:string):string {return x;}`)
	requireExternClean(t, r)
	_, err := emitABAP(t, r.Prog)
	if err == nil || !strings.Contains(err.Error(), "extern module call Basename is Go-only") || !strings.Contains(err.Error(), "probe.ts:") {
		t.Fatal(err)
	}
}

func buildExternProbe(t *testing.T, r *RegistryLowering, modules []ExternModule) (string, string) {
	t.Helper()
	requireExternClean(t, r)
	files, err := gohir.Emit(r.Prog)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files["go.mod"] = ExternGoMod("probe", modules, dir)
	files["main.go"] = fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Println(%s())}\n", goEntry("probe.ts.Probe", "run"))
	for n, s := range files {
		if err = os.WriteFile(filepath.Join(dir, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "probe")
	cmd := exec.Command("nice", "go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %v: %s", err, b)
	}
	return bin, dir
}
func TestExternTypedABI(t *testing.T) {
	source := `import * as host from './host';class Probe {
 static run():string {
  let out=host.text("😀")+"|"+host.flip(true)+"|"+host.add(40,2)+"|"+host.strings(["a","😀"]).join(",");
  out+="|"+host.join("a",...["b"])+"|"+host.maybe(true)+"|"+(host.maybe(false)===undefined);
  out+="|"+host.tuples(host.entries());
  try {host.fail();} catch(e) {out+="|"+e.message;}
  host.nothing();return out;
 }} `
	host := `export function text(x:string):string{return x;}
 export function flip(x:boolean):boolean{return x;}
 export function add(x:number,y:number):number{return x+y;}
 export function strings(x:string[]):string[]{return x;}
 export function join(...x:string[]):string{return "";}
 export function maybe(x:boolean):string|undefined{return undefined;}
 export function entries():[string,boolean][]{return [];}
 export function tuples(x:[string,boolean][]):string{return "";}
 export function fail():void{}
 export function nothing():void{}`
	goHost := `package hostio
 import ("errors";"strings")
 func Text(x string) string {return x}
 func Flip(x bool) (bool,error) {return !x,nil}
 func Add(x,y float64) float64 {return x+y}
 func Strings(x []string) ([]string,error) {return x,nil}
 func Join(x ...string) (string,error) {return strings.Join(x,"/"),nil}
 func Maybe(x bool) (string,bool,error) {return "",x,nil}
 func Entries() []struct{F0 string;F1 bool} {return []struct{F0 string;F1 bool}{{"entry",true}}}
 func Tuples(x []struct{F0 string;F1 bool}) string {if x[0].F1{return x[0].F0};return "bad"}
 func Fail() error {return errors.New("host failed")}
 func Nothing() {}`
	for _, assume := range []bool{false, true} {
		t.Run(fmt.Sprintf("assumeInt=%v", assume), func(t *testing.T) {
			r, modules, dir := externProject(t, source, host)
			pkg := t.TempDir()
			modules[0].GoDir = pkg
			for n, s := range map[string]string{"go.mod": "module " + modules[0].GoImport + "\n\ngo 1.26.0\n", "host.go": goHost} {
				if err := os.WriteFile(filepath.Join(pkg, n), []byte(s), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if assume {
				registry, _ := overrides.New()
				var err error
				r, err = LowerProjectWithOptions(filepath.Join(dir, "tsconfig.json"), []string{"probe.ts", "host.ts"}, registry, nil, LowerOptions{Externs: modules, AssumeOnlyIntegerCalculations: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			bin, build := buildExternProbe(t, r, modules)
			cmd := exec.Command("nice", bin)
			b, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run %v %s", err, b)
			}
			if string(b) != "😀|false|42|a,😀|a/b||true|entry|host failed\n" {
				t.Fatalf("output %q", b)
			}
			for _, bad := range []string{strings.Replace(goHost, "Text(x string) string {return x}", "Text(x bool) string {return \"bad\"}", 1), strings.Replace(goHost, "Flip(x bool) (bool,error) {return !x,nil}", "Flip(x bool) (bool,bool) {return x,x}", 1)} {
				if err := os.WriteFile(filepath.Join(pkg, "host.go"), []byte(bad), 0600); err != nil {
					t.Fatal(err)
				}
				cmd = exec.Command("nice", "go", "build", "-o", bin, ".")
				cmd.Dir = build
				cmd.Env = append(os.Environ(), "GOWORK=off")
				b, err = cmd.CombinedOutput()
				if err == nil || !strings.Contains(string(b), "does not satisfy") {
					t.Fatalf("ABI mismatch was not rejected: %v %s", err, b)
				}
			}
		})
	}
}

func TestExternArrayInputContract(t *testing.T) {
	for _, file := range []string{"../docs/extern-modules.md", "../extern/hostio/hostio.go"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, contract := range []string{"read-only inputs", "not written back"} {
			if !strings.Contains(string(b), contract) {
				t.Errorf("%s does not document %q", file, contract)
			}
		}
	}
}

func TestExternLoneSurrogateArguments(t *testing.T) {
	host := `export function text(x:string):string{return x;}
 export function strings(x:string[]):string{return "";}
 export function tuples(x:[string,boolean][]):string{return "";}
 export function join(...x:string[]):string{return "";}`
	for _, call := range []string{`host.text("😀".slice(0,1))`, `host.text("😀".slice(1))`, `host.strings(["😀".slice(0,1)])`, `host.tuples([["😀".slice(1),true]])`, `host.join("😀".slice(0,1))`, `host.join(...["😀".slice(1)])`} {
		t.Run(call, func(t *testing.T) {
			r, modules, _ := externProject(t, `import * as host from './host';class Probe {static run():string {try {return `+call+`;} catch(e) {return "caught";}}}`, host)
			pkg := t.TempDir()
			modules[0].GoDir = pkg
			for n, s := range map[string]string{"go.mod": "module " + modules[0].GoImport + "\n\ngo 1.26.0\n", "host.go": `package hostio
func Text(x string) string {return x}
func Strings(x []string) string {return "called"}
func Tuples(x []struct{F0 string;F1 bool}) string {return "called"}
func Join(x ...string) string {return "called"}`} {
				if err := os.WriteFile(filepath.Join(pkg, n), []byte(s), 0600); err != nil {
					t.Fatal(err)
				}
			}
			bin, _ := buildExternProbe(t, r, modules)
			b, err := exec.Command("nice", bin).CombinedOutput()
			if err == nil || !strings.Contains(string(b), "probe.ts:") || !strings.Contains(string(b), "extern string argument has a lone surrogate") {
				t.Fatalf("expected located extern string trap, got %v: %s", err, b)
			}
		})
	}
}

func TestExternShadowedNamespaceStaysExcluded(t *testing.T) {
	r, _, _ := externProject(t,
		`import * as io from "./host"; class Probe { static run():string { const io="ok"; return io; } }`,
		`const marker=new Date(); export function f():string{return "";}`)
	requireExternClean(t, r)
	if dump := hir.Dump(r.Prog); strings.Contains(dump, "marker") {
		t.Fatalf("extern initializer entered HIR:\n%s", dump)
	}
}
