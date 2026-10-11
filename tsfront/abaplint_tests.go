package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
)

// LowerAbaplintTests verifies and lowers a tools/abaplint-tests-closure.mjs
// closure. Failed bodies trap; no coverage manifest or workload pruning.
func LowerAbaplintTests(dir string) (*RegistryLowering, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		return nil, err
	}
	closure, err := ParseRegistryClosure(raw)
	if err != nil {
		return nil, err
	}
	if err = closure.Verify(dir); err != nil {
		return nil, err
	}
	registry, err := RegistryOverrides()
	if err != nil {
		return nil, err
	}
	// Unpruned tests use binary64 number semantics, not a workload-specific
	// assume-integer contract. Ignore ABAPITI_ASSUME_INT deliberately.
	return LowerRegistryWithOptions(dir, closure.Files(), registry, nil, LowerOptions{TrapOnFailure: true})
}

// EmitAbaplintTests emits verified programs, including failure-trapped bodies.
// Unattributable structural or backend failures still refuse emission.
func EmitAbaplintTests(dir string, lowering *RegistryLowering) (map[string]string, error) {
	if err := lowering.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Initialization []string `json:"initialization"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	if len(manifest.Initialization) == 0 {
		return nil, fmt.Errorf("test closure has no initialization order")
	}
	modules := map[string]bool{}
	runtime := false
	for _, c := range lowering.Prog.Classes {
		modules[c.Name] = true
		if c.Name == "runtime/mocha.ts.TestRuntime" {
			runtime = true
		}
	}
	if !runtime {
		return nil, fmt.Errorf("test closure has no TestRuntime")
	}
	names := hir.NewNames()
	var init strings.Builder
	for _, file := range manifest.Initialization {
		if modules[file+".module"] {
			fmt.Fprintf(&init, "initialize(%q,func(){%s()})\n", file, names.Get("init."+file+".module"))
		}
	}
	var files map[string]string
	for {
		files, err = gohir.Emit(lowering.Prog)
		if err == nil {
			break
		}
		e, ok := err.(hir.Error)
		if !ok || e.ID == 0 {
			return nil, err
		}
		trapped := false
		for _, c := range lowering.Prog.Classes {
			for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
				if m != nil && bodyIDs(m.Body)[e.ID] {
					m.Body = failureBody(m.Node, e.Error())
					trapped = true
				}
			}
		}
		if !trapped {
			return nil, err
		}
		(&lowerer{out: lowering.Prog}).repairFailedBodies()
		if errors := hir.Verify(lowering.Prog); len(errors) > 0 {
			return nil, fmt.Errorf("after Go failure trap: %s", errors[0])
		}
	}
	files["go.mod"] = "module abaplinttests\n\ngo 1.26.0\n"
	message := "message:=func(x any) string {return fmt.Sprint(x)}"
	if modules["builtin.Error"] {
		message = fmt.Sprintf("message:=func(x any) string {if p,ok:=x.(payload[%s]);ok && p.Value!=nil{return p.Value.%s().%s.String()};return fmt.Sprint(x)}", names.Get("ref.builtin.Error"), names.Get("base.builtin.Error"), names.Get("member.message"))
	}
	files["main.go"] = fmt.Sprintf(`package main
import ("encoding/json"; "fmt"; "os"; "strings")
func main() {
 defer func() { if x := recover(); x != nil { fmt.Fprintf(os.Stderr, "test harness refused: %%v\n", x); os.Exit(2) } }()
 initialize:=func(file string, f func()){defer func(){if x:=recover();x!=nil{fmt.Fprintf(os.Stderr,"initialization %%s: %%v\n",file,x)}}();f()}
 %s
 %s()
 %s
 type record struct { Title string `+"`json:\"title\"`"+`; Passed bool `+"`json:\"passed\"`"+`; Message string `+"`json:\"message\"`"+`; Status string `+"`json:\"status\"`"+` }
 summary := struct { Passed int `+"`json:\"passed\"`"+`; Failed int `+"`json:\"failed\"`"+`; Trapped int `+"`json:\"trapped\"`"+`; NotRun int `+"`json:\"notRun\"`"+`; Results []record `+"`json:\"results\"`"+` }{Results:[]record{}}
 for _, item := range %s.Items {
  t := item.(%s)
  r:=record{Title:t.%s.String(),Status:"passed"}
  if t.%s || (%s && !t.%s) {r.Status="not-run";summary.NotRun++} else {
   func(){defer func(){if x:=recover();x!=nil{
    r.Message=message(x); r.Status="failed"
    if _,ok:=x.(trap);ok {r.Status="trapped"}
    if strings.Contains(r.Message,"unsupported-async") {r.Status="not-run"}
   }}();t.%s.%s()}()
   switch r.Status {case "passed":r.Passed=true;summary.Passed++;case "trapped":summary.Trapped++;case "not-run":summary.NotRun++;default:summary.Failed++}
  }
  summary.Results=append(summary.Results,r)
  if err:=json.NewEncoder(os.Stderr).Encode(r);err!=nil{panic(err)}
 }
 if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil { panic(err) }
 if summary.Failed + summary.Trapped != 0 { os.Exit(1) }
}
`, init.String(), names.Get("init.runtime/mocha.ts.TestRuntime"), message, names.Get("static.runtime/mocha.ts.TestRuntime.tests"), names.Get("ref.runtime/mocha.ts.TestCase"), names.Get("member.title"), names.Get("member.skipped"), names.Get("static.runtime/mocha.ts.TestRuntime.focused"), names.Get("member.only"), names.Get("member.body"), names.Get("member.call"))
	return files, nil
}
