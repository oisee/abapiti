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
)

// A deliberately observable shared mutation must be detected independently by
// both checks. The same receiver temporary must still update the TS local.
func TestResultDebugSharing(t *testing.T) {
	n := hir.NewNames()
	obj := n.Get("struct." + resultClass)
	body := func(m string) string { return n.Get("body." + resultClass + "." + m) }
	fixture := fmt.Sprintf(`package main
import "fmt"
type %s struct{N int}
func(self *%s) z_base_fixture()*%s{return self}
var staticResult *%s
var goSources=map[string]string{"main":"fixture.ts:1:1"}
func %s(self *%s){self.z_base_fixture().N++}
func %s(self *%s){self.z_base_fixture().N++}
func %s(self *%s){self.z_base_fixture().N++}
func main(){
 var a *%s = &%s{N:7}
 var b *%s = a
 var box *array[any] = &array[any]{}
 box.push(a)
 staticResult=a
 var receiver *%s = a
 resultDebugMutation("fixture.ts:10:3","a")
 %s(receiver)
 fmt.Printf("%%d %%d %%d %%d\n",a.N,b.N,box.Items[0].(*%s).N,staticResult.N)
}
`, obj, obj, obj, obj, body("wrapConsumed"), obj, body("popNode"), obj, body("setNodes"), obj, obj, obj, obj, obj, body("wrapConsumed"), obj)
	runtime := `package main
type array[T any]struct{Items []T}
func(a *array[T])push(v T){a.Items=append(a.Items,v)}
func(a *array[T])put(i int,v T){a.Items[i]=v}
type orderedMap[K comparable,V any]struct{Values map[K]V}
func(m *orderedMap[K,V])set(k K,v V){m.Values[k]=v}
`
	metadata, _ := json.Marshal(map[string]map[string]gohir.ResultDebugLocal{"main": {
		"a": {Root: true, Name: "a", Source: "fixture.ts:2:1"}, "b": {Root: true, Name: "b", Source: "fixture.ts:3:1"}, "box": {Root: true, Name: "box", Source: "fixture.ts:4:1"},
	}})
	for _, mode := range []string{"ownership", "value"} {
		t.Run(mode, func(t *testing.T) {
			files := map[string]string{"hir.go": fixture, "runtime.go": runtime, "main.go": "package main\n", "result-debug-locals.json": string(metadata)}
			// The fixture's main lives in hir.go. This is also useful for a synthetic
			// non-CLI module: invoke report directly after main exits in the test host.
			if mode == "ownership" {
				files["hir.go"] = strings.Replace(fixture, "func main(){", "func main(){defer resultReport();", 1)
			}
			// Type checking occurs before runtime insertion, so declare the test-only
			// report signature alongside the normal input and strip it afterwards.
			if mode == "ownership" {
				files["main.go"] = "package main\nfunc resultReport(){}\n"
			}
			if err := RegistryGoResultDebug(files, mode); err != nil {
				t.Fatal(err)
			}
			files["main.go"] = "package main\n"
			dir := t.TempDir()
			for name, src := range files {
				if strings.HasSuffix(name, ".go") {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module resultfixture\ngo 1.26.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "run", ".")
			cmd.Dir = dir
			report := filepath.Join(dir, "report.json")
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false", "ABAPITI_RESULT_REPORT="+report)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			expected := "8 8 8 8\n"
			if mode == "value" {
				expected = "8 7 7 7\n"
			}
			if string(output) != expected {
				t.Fatalf("got %q, want %q", output, expected)
			}
			if mode == "ownership" {
				raw, err := os.ReadFile(report)
				if err != nil {
					t.Fatal(err)
				}
				var finding struct {
					Checks     int
					Violations []struct {
						Mutator, Site string
						Holders       []struct{ Path, Created string }
					}
				}
				if err := json.Unmarshal(raw, &finding); err != nil {
					t.Fatal(err)
				}
				if finding.Checks != 1 || len(finding.Violations) != 1 {
					t.Fatalf("unexpected report: %s", raw)
				}
				v := finding.Violations[0]
				if v.Mutator != "wrapConsumed" || v.Site != "fixture.ts:10:3" || len(v.Holders) < 4 {
					t.Fatalf("missing holder/source evidence: %s", raw)
				}
			}
		})
	}
}

func TestResultDebugFailsClosed(t *testing.T) {
	for _, mode := range []string{"", "value", "ownership", "wrong"} {
		if err := RegistryGoResultDebug(map[string]string{}, mode); err == nil {
			t.Fatalf("mode %q accepted missing debug metadata", mode)
		}
	}
}

func TestResultRuntimeCurrentHolders(t *testing.T) {
	obj := hir.NewNames().Get("struct." + resultClass)
	runtime := strings.ReplaceAll(strings.ReplaceAll(debugRuntime, "@OBJ@", obj), "@SHAPES@", `"{}"`)
	testSource := strings.ReplaceAll(`package main
import "testing"
type @OBJ@ struct{N int}
func TestHolders(t *testing.T){
 r:=&@OBJ@{};f:=resultEnter("holders.ts:1:1");defer resultLeave(f)
 resultRoot(f,"r",func()any{return r},"local",100,1);resultAt(f,2)
 check:=func(){resultDebugMutation("holders.ts:2:1","r");resultCheck(r,"wrapConsumed")}
 check();if len(resultFindings)!=0{t.Fatal("sole receiver is not shared")}
 box:=[]any{r};alias:=box
 resultRoot(f,"box",func()any{return box},"array store",100,1)
 resultRoot(f,"alias",func()any{return alias},"container alias",100,1)
 check();if len(resultFindings)!=1{t.Fatal("missing collection holder")};for _,v:=range resultFindings{if len(v.Holders)!=2{t.Fatal("container aliases double counted",v.Holders)}}
 box[0]=nil;check();for _,v:=range resultFindings{if v.Count!=1{t.Fatal("removed element still counted")}}
 field:=struct{R *@OBJ@}{r};resultRoot(f,"field",func()any{return field},"field store",100,1);check()
 if len(resultFindings)!=2{t.Fatal("missing object field")};field.R=nil
 m:=map[string]*@OBJ@{"key":r};resultRoot(f,"map",func()any{return m},"map store",100,1)
 resultRoot(f,"mapAlias",func()any{return m},"map alias",100,1);check()
 if len(resultFindings)!=3{t.Fatal("missing map entry")};for _,v:=range resultFindings{if len(v.Holders)!=2{t.Fatal("map aliases double counted")}}
 delete(m,"key");check();if len(resultFindings)!=3{t.Fatal("deleted map entry remained live")}
 keymap:=map[*@OBJ@]int{r:1};resultRoot(f,"keymap",func()any{return keymap},"map key",100,1);check();if len(resultFindings)!=4{t.Fatal("Result map key not counted")};delete(keymap,r)
 resultRoot(f,"dead",func()any{return r},"dead local",1,1);check();if len(resultFindings)!=4{t.Fatal("dead local counted")}
 resultPublish(r,"untracked closure capture");check();if len(resultFindings)!=5{t.Fatal("published Result accepted")}
 q:=&@OBJ@{};resultPublish(&struct{hidden *@OBJ@}{q},"hidden field capture");resultDebugMutation("holders.ts:3:1","");resultCheck(q,"setNodes");if len(resultFindings)!=6{t.Fatal("unexported capture was missed")}
}
`, "@OBJ@", obj)
	dir := t.TempDir()
	for name, src := range map[string]string{"result_debug.go": runtime, "result_debug_test.go": testSource, "go.mod": "module holderfixture\ngo 1.26.0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
