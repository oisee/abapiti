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
// closure. No coverage manifest is supplied: every body must translate.
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
	return LowerRegistryWithOptions(dir, closure.Files(), registry, nil, LowerOptions{})
}

// EmitAbaplintTests only emits verified, complete programs. A failing closure
// is evidence of missing lowering, not a set of failed mocha tests.
func EmitAbaplintTests(dir string, lowering *RegistryLowering) (map[string]string, error) {
	if err := lowering.Err(); err != nil {
		return nil, err
	}
	if lowering.TrapCount() != 0 {
		return nil, fmt.Errorf("test closure contains coverage traps")
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
			fmt.Fprintf(&init, "%s()\n", names.Get("init."+file+".module"))
		}
	}
	files, err := gohir.Emit(lowering.Prog)
	if err != nil {
		return nil, err
	}
	files["go.mod"] = "module abaplinttests\n\ngo 1.26.0\n"
	files["main.go"] = fmt.Sprintf(`package main
import ("encoding/json"; "fmt"; "os")
func main() {
 defer func() { if x := recover(); x != nil { fmt.Fprintf(os.Stderr, "test harness refused: %%v\n", x); os.Exit(2) } }()
 %s
 result := %s()
 type failure struct { Title string `+"`json:\"title\"`"+`; Message string `+"`json:\"message\"`"+` }
 summary := struct { Passed int `+"`json:\"passed\"`"+`; Failed int `+"`json:\"failed\"`"+`; Failures []failure `+"`json:\"failures\"`"+` }{Passed:int(result.%s), Failed:int(result.%s), Failures:[]failure{}}
 for _, item := range result.%s.Items {
  f := item.(%s)
  summary.Failures = append(summary.Failures, failure{Title:f.%s.String(), Message:f.%s.String()})
 }
 for _, item := range %s.Items {
  r := item.(%s)
  record := struct { Title string `+"`json:\"title\"`"+`; Passed bool `+"`json:\"passed\"`"+`; Message string `+"`json:\"message\"`"+` }{r.%s.String(),r.%s,r.%s.String()}
  if err := json.NewEncoder(os.Stderr).Encode(record); err != nil { panic(err) }
 }
 if summary.Passed + summary.Failed == 0 { panic("no tests executed") }
 if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil { panic(err) }
 if summary.Failed != 0 { os.Exit(1) }
}
`, init.String(), names.Get("body.runtime/mocha.ts.TestRuntime.run"), names.Get("member.passed"), names.Get("member.failed"), names.Get("member.failures"), names.Get("ref.runtime/mocha.ts.TestFailure"), names.Get("member.title"), names.Get("member.message"), names.Get("static.runtime/mocha.ts.TestRuntime.results"), names.Get("ref.runtime/mocha.ts.TestResult"), names.Get("member.title"), names.Get("member.passed"), names.Get("member.message"))
	return files, nil
}
