package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

const closureStructuralTypes = `interface SourceFile {label:string}
interface ClassObject { getName():string; getMainABAPFile():SourceFile|undefined; obj?:string }
interface LocalClass { getName():string; getMainABAPFile():SourceFile; obj?:string; extra():string }
type Compile = (obj:ClassObject)=>ClassObject;
function compile(obj:ClassObject):ClassObject {obj.obj="written";return obj;}
class Probe {
 static same(a:unknown,b:unknown):boolean{return a===b;}
 static flag=true;
 static calls=0;
 static input():LocalClass {Probe.calls++;return {getName:()=>"local",getMainABAPFile:()=>({label:"file"}),extra:()=>"extra"};}
 static run():string {
 const local=Probe.input();
 const call:Compile=Probe.flag ? (obj)=>compile(obj) : (obj)=>obj;
 const result=call(local);
 const before=local.obj;
 const same=Probe.same(result,local);
 local.obj="late";
 const direct=compile(local);
 result.obj="alias";
 return same+":"+before+":"+result.getName()+":"+result.getMainABAPFile()?.label+":"+result.obj+":"+local.obj+":"+direct.obj+":"+Probe.calls;
}}`

const closureStructuralJS = `/** @param {import("./types.ts").ClassObject} obj @returns {import("./types.ts").ClassObject} */
function compile(obj){obj.obj="written";return obj;}
class Probe {
 /** @param {unknown} a @param {unknown} b @returns {boolean} */ static same(a,b){return a===b;}
 static flag=true; static calls=0;
 /** @returns {import("./types.ts").LocalClass} */ static input(){Probe.calls++;return {getName:()=>"local",getMainABAPFile:()=>({label:"file"}),extra:()=>"extra"};}
 /** @returns {string} */ static run(){
 const local=Probe.input();
 /** @type {import("./types.ts").Compile} */ const call=Probe.flag ? (obj)=>compile(obj) : (obj)=>obj;
 const result=call(local);const before=local.obj;const same=Probe.same(result,local);
 local.obj="late";const direct=compile(local);result.obj="alias";
 return same+":"+before+":"+result.getName()+":"+result.getMainABAPFile()?.label+":"+result.obj+":"+local.obj+":"+direct.obj+":"+Probe.calls;
}}`

// Both a stored closure and a named call must use the checked live argument
// view. Method results widen from present to optional references; the optional
// data slot and original object remain shared in both directions.
func TestStep4ClosureStructuralArgument(t *testing.T) {
	for _, js := range []bool{false, true} {
		t.Run(fmt.Sprintf("js=%v", js), func(t *testing.T) {
			dir := t.TempDir()
			if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
				dir = filepath.Join(base, t.Name())
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			file, source := "probe.ts", closureStructuralTypes
			if js {
				file, source = "probe.js", closureStructuralJS
			}
			types := `export interface SourceFile {label:string}
export interface ClassObject {getName():string;getMainABAPFile():SourceFile|undefined;obj?:string}
export interface LocalClass {getName():string;getMainABAPFile():SourceFile;obj?:string;extra():string}
export type Compile=(obj:ClassObject)=>ClassObject;`
			for name, text := range map[string]string{file: source, "types.ts": types, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","allowJs":true,"checkJs":true,"noEmit":true},"files":["` + file + `"]}`} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// Check the fixtures with upstream strict TypeScript, including checkJs.
			out, err := exec.Command("node", filepath.Join(typescriptModule(t), "bin/tsc"), "-p", filepath.Join(dir, "tsconfig.json")).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(out))
			}
			want := conversionOracle(t, source)
			if want != "true:written:local:file:alias:alias:alias:1\n" {
				t.Fatal(want)
			}
			project, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			files := []string{file}
			if js {
				files = append(files, "types.ts")
			}
			p, ds, err := project.Lower(files)
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(p); len(errs) > 0 {
				t.Fatal(errs)
			}
			// Ensure this fixture exercises the stored callable ABI, not only a lifted
			// direct arrow. The named compile(local) below supplies the direct control.
			if !regexp.MustCompile(`virtual .*\.call\(`).MatchString(hir.Dump(p)) {
				t.Fatal("missing virtual closure invocation")
			}
			owner := file + ".Probe"
			if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry(owner, "run"))); got != want {
				t.Fatalf("Go %q Node %q", got, want)
			}
			// ABAP closure interfaces are nominal even when their reference result
			// ABIs coincide. Exercise the live view and both call paths there,
			// reading the covariant method through its original source slot.
			abapSource := strings.Replace(source, "result.getMainABAPFile()?.label", "local.getMainABAPFile().label", 1)
			if err := os.WriteFile(filepath.Join(dir, file), []byte(abapSource), 0600); err != nil {
				t.Fatal(err)
			}
			abapProject, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			abapProgram, ds, err := abapProject.Lower(files)
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(abapProgram); len(errs) > 0 {
				t.Fatal(errs)
			}
			storageABAPProbe(t, abapProgram, owner, strings.TrimSpace(want), "")
			if err := os.WriteFile(filepath.Join(dir, "go-"+file), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A private backing member is not a public structural contract, even when
// the destination marks that member optional. Keep both call paths refused.
func TestStep4ClosureStructuralArgumentRejectsPrivateMember(t *testing.T) {
	source := `interface Payload {def?:unknown}
class NativePayload {private def=1;}
interface Wanted {getName():string;obj?:Payload}
interface Actual {getName():string;obj:NativePayload}
type Invoke=(obj:Wanted)=>string;
function direct(obj:Wanted):string{return obj.getName();}
class Probe {static flag=true;static run():string {
 const actual:Actual={getName:()=>"local",obj:new NativePayload()};
 const invoke:Invoke=Probe.flag?(obj)=>direct(obj):(obj)=>obj.getName();
 return invoke(actual)+direct(actual);
}}`
	p, ds := lowerProbe(t, sourceProbe(t, source))
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	var virtual, direct bool
	for _, err := range hir.Verify(p) {
		message := fmt.Sprint(err)
		if strings.Contains(message, "call argument type mismatch") {
			virtual = virtual || strings.Contains(message, "virtual .call")
			direct = direct || strings.Contains(message, "module.direct")
		}
	}
	if !virtual || !direct {
		t.Fatalf("private contract must remain refused on both paths: %v", hir.Verify(p))
	}
}
