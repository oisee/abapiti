package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestExternCLI(t *testing.T) {
	dir := t.TempDir()
	pkg, err := filepath.Abs("../../extern/hostio")
	if err != nil {
		t.Fatal(err)
	}
	for n, s := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2022"},"files":["probe.ts","host.ts"]}`,
		"host.ts":       `export function basename(x:string):string {console.log("body must not lower");return x;}`,
		"probe.ts":      `import {basename} from './host';class Probe {static run():string{return basename("/a/b/");}}`,
		"externs.json":  fmt.Sprintf(`{"modules":[{"file":"host.ts","goImport":"github.com/oisee/abapiti/extern/hostio","goDir":%q}]}`, pkg),
	} {
		if err = os.WriteFile(filepath.Join(dir, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"--tsconfig", filepath.Join(dir, "tsconfig.json"), "--externs", filepath.Join(dir, "externs.json"), "-o", filepath.Join(dir, "out"), "probe.ts", "host.ts"}
	lower := newLowerCmd()
	lower.SetArgs(append([]string{"--full"}, args...))
	if err = lower.Execute(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "out", "evidence", "gap-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"blocking": 0`) {
		t.Fatal(string(raw))
	}
	emit := newEmitGoCmd()
	emit.SetArgs(append([]string{"--build", "--entry", "probe.ts.Probe.run"}, args...))
	if err = emit.Execute(); err != nil {
		t.Fatal(err)
	}
	main := fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Println(%s().String())}\n", hir.NewNames().Get("body.probe.ts.Probe.run"))
	if err = os.WriteFile(filepath.Join(dir, "out", "main.go"), []byte(main), 0600); err != nil {
		t.Fatal(err)
	}
	process := exec.Command("nice", "go", "run", ".")
	process.Dir = filepath.Join(dir, "out")
	process.Env = append(os.Environ(), "GOWORK=off")
	b, err := process.CombinedOutput()
	if err != nil || string(b) != "b\n" {
		t.Fatalf("%v %s", err, b)
	}
}
