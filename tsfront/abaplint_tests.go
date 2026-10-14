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
	return LowerAbaplintTestsWithCore(dir, false)
}

// LowerAbaplintTestsWithCore includes the deployment harness in the same
// unpruned program as the tests, so both clients share one name table and core.
func LowerAbaplintTestsWithCore(dir string, fullCore bool) (*RegistryLowering, error) {
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
	files := closure.Files()
	if fullCore {
		path := filepath.Join(dir, RegistryHarnessPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, RegistryHarness(), 0644); err != nil {
			return nil, err
		}
		files = append(files, RegistryHarnessPath)
		configPath := filepath.Join(dir, "tsconfig.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		var config map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
		include, _ := config["include"].([]any)
		config["include"] = append(include, "harness/**/*.ts")
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, err
		}
	}
	return LowerRegistryWithOptions(dir, files, registry, nil, LowerOptions{TrapOnFailure: true})
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
	actions := map[string][]*hir.Method{}
	runtime := false
	for _, c := range lowering.Prog.Classes {
		modules[c.Name] = true
		for _, m := range c.Methods {
			if isTestInitialization(m) {
				actions[c.Name] = append(actions[c.Name], m)
			}
		}
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
			fmt.Fprintf(&init, "initialize(%q,%q,func(){%s()})\n", file, "module", names.Get("init."+file+".module"))
			for _, action := range actions[file+".module"] {
				fmt.Fprintf(&init, "initialize(%q,%q,func(){%s()})\n", file, action.Source, names.Get("body."+file+".module."+action.Name))
			}
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
	// Async rejection preserves a typed TS payload. Unwrap primitive reasons
	// and Error subclasses before formatting the report message.
	message := "message:=func(x any) string {if p,ok:=x.(interface{tsValue() *dynamic});ok {d:=p.tsValue();if d==nil{return \"undefined\"};x=d.Value};ERROR_MESSAGE;return fmt.Sprint(x)}"
	errorMessage := ""
	if modules["builtin.Error"] {
		errorMessage = fmt.Sprintf("if p,ok:=x.(%s);ok && p!=nil{return p.%s().%s.String()}", names.Get("ref.builtin.Error"), names.Get("base.builtin.Error"), names.Get("member.message"))
	}
	message = strings.ReplaceAll(message, "ERROR_MESSAGE", errorMessage)
	files["main.go"] = fmt.Sprintf(`package main
import ("encoding/json"; "fmt"; "os"; "strings")
func main() {
 defer func() { if x := recover(); x != nil { fmt.Fprintf(os.Stderr, "test harness refused: %%v\n", x); os.Exit(2) } }()
 %s
 type initFailure struct { File string `+"`json:\"file\"`"+`; Action string `+"`json:\"action\"`"+`; Message string `+"`json:\"message\"`"+`; Status string `+"`json:\"status\"`"+` }
 initialization:=[]initFailure{}
 initialize:=func(file, action string, f func()){defer func(){if x:=recover();x!=nil{
  status:="failed";switch x.(type) {case trap, rangeFault: status="trapped"}
  r:=initFailure{file,action,message(x),status};initialization=append(initialization,r)
  fmt.Fprintf(os.Stderr,"initialization %%s (%%s): %%s\n",file,action,r.Message)
 }}();f()}
 %s
 %s()
 type record struct { Title string `+"`json:\"title\"`"+`; Passed bool `+"`json:\"passed\"`"+`; Message string `+"`json:\"message\"`"+`; Status string `+"`json:\"status\"`"+` }
 summary := struct { AsyncPending int `+"`json:\"asyncPending\"`"+`; Passed int `+"`json:\"passed\"`"+`; Failed int `+"`json:\"failed\"`"+`; Trapped int `+"`json:\"trapped\"`"+`; NotRun int `+"`json:\"notRun\"`"+`; Results []record `+"`json:\"results\"`"+`; Initialization []initFailure `+"`json:\"initialization\"`"+` }{Results:[]record{}, Initialization:initialization}
 %s
 for _, item := range %s.Items {
  t := item.(%s)
  r:=record{Title:t.%s.String(),Status:"passed"}
  if t.%s || (%s && !t.%s) {r.Status="not-run";summary.NotRun++} else {
   apply:=func(x any) {if x==nil || r.Status!="passed" {return};r.Message=message(x);r.Status="failed";switch x.(type) {case rangeFault:r.Status="trapped";case trap:r.Status="trapped";if strings.Contains(r.Message,"unsupported-async") || strings.Contains(r.Message,"certified settled promise") {r.Status="async-pending"};if strings.Contains(r.Message,"unsupported-test-done") {r.Status="not-run"}}}
   recoverCall:=func(f func()) (failure any) {defer func(){failure=recover()}();f();return nil}
   %s
   switch r.Status {case "async-pending":summary.AsyncPending++;case "passed":r.Passed=true;summary.Passed++;case "trapped":summary.Trapped++;case "not-run":summary.NotRun++;default:summary.Failed++}
  }
  summary.Results=append(summary.Results,r)
  if err:=json.NewEncoder(os.Stderr).Encode(r);err!=nil{panic(err)}
 }
 if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil { panic(err) }
 if summary.Failed + summary.Trapped + summary.AsyncPending != 0 { os.Exit(1) }
}
`, message, init.String(), names.Get("init.runtime/mocha.ts.TestRuntime"), testHookSetup(names, modules), names.Get("static.runtime/mocha.ts.TestRuntime.tests"), names.Get("ref.runtime/mocha.ts.TestCase"), names.Get("member.title"), names.Get("member.skipped"), names.Get("static.runtime/mocha.ts.TestRuntime.focused"), names.Get("member.only"), testHookExecution(names, modules))
	return files, nil
}

// Hook failures use the same recovery boundary as bodies. A failed before hook
// is replayed for every affected test; afterEach and after still get drained.
func testHookSetup(names *hir.Names, modules map[string]bool) string {
	if !modules["runtime/mocha.ts.TestSuite"] {
		return ""
	}
	return testHookNames(names, `
 remaining:=map[SUITE]int{}
 started:=map[SUITE]bool{}
 beforeFailure:=map[SUITE]any{}
 for _,item:=range TESTS.Items {t:=item.(CASE);if t.SKIPPED || (FOCUSED && !t.ONLY) {continue};for _,entry:=range t.SUITES.Items {remaining[entry.(SUITE)]++}}
 `)
}
func testHookExecution(names *hir.Names, modules map[string]bool) string {
	if !modules["runtime/mocha.ts.TestSuite"] {
		return testHookNames(names, `apply(recoverCall(func(){drainSettled(t.BODY.CALL())}))`)
	}
	return testHookNames(names, `
 entered:=[]SUITE{}
 apply(recoverCall(func(){
  for _,entry:=range t.SUITES.Items {
   suite:=entry.(SUITE)
   if !started[suite] {started[suite]=true;beforeFailure[suite]=recoverCall(func(){for _,h:=range suite.BEFOREALL.Items {drainSettled(h.(THUNK).CALL())}})}
   if x:=beforeFailure[suite];x!=nil {panic(x)}
   entered=append(entered,suite)
   for _,h:=range suite.BEFOREEACH.Items {drainSettled(h.(THUNK).CALL())}
  }
  drainSettled(t.BODY.CALL())
 }))
 for i:=len(entered)-1;i>=0;i-- {for _,h:=range entered[i].AFTEREACH.Items {apply(recoverCall(func(){drainSettled(h.(THUNK).CALL())}))}}
 for i:=len(t.SUITES.Items)-1;i>=0;i-- {suite:=t.SUITES.Items[i].(SUITE);remaining[suite]--;if remaining[suite]==0 && started[suite] {for _,h:=range suite.AFTERALL.Items {apply(recoverCall(func(){drainSettled(h.(THUNK).CALL())}))}}}
 `)
}
func testHookNames(names *hir.Names, source string) string {
	return strings.NewReplacer(
		"SUITES", names.Get("member.suites"), "SUITE", names.Get("ref.runtime/mocha.ts.TestSuite"),
		"CASE", names.Get("ref.runtime/mocha.ts.TestCase"),
		"THUNK", names.Get("ref.closure.thunk"),
		"TESTS", names.Get("static.runtime/mocha.ts.TestRuntime.tests"),
		"FOCUSED", names.Get("static.runtime/mocha.ts.TestRuntime.focused"),
		"SKIPPED", names.Get("member.skipped"), "ONLY", names.Get("member.only"),
		"BODY", names.Get("member.body"), "CALL", names.Get("member.call"),
		"BEFOREALL", names.Get("member.beforeAll"), "AFTERALL", names.Get("member.afterAll"),
		"BEFOREEACH", names.Get("member.beforeEach"), "AFTEREACH", names.Get("member.afterEach"),
	).Replace(source)
}
