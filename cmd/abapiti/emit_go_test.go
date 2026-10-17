package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitGoFailureTrapsAndModuleInitialization(t *testing.T) {
	dir := t.TempDir()
	host, err := filepath.Abs("../../extern/hostio")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "initialized.txt")
	for name, text := range map[string]string{
		"tsconfig.json":       `{"compilerOptions":{"strict":true,"target":"es2022"},"files":["probe.ts","host.ts"]}`,
		"initialization.json": `["host.ts","probe.ts"]`,
		"host.ts":             `export declare function writeText(file:string,text:string):void;`,
		"probe.ts":            fmt.Sprintf(`import {writeText} from './host'; writeText(%q,"initialized"); class Probe {static bad():void {eval("unreviewed");}}`, marker),
		"externs.json":        fmt.Sprintf(`{"modules":[{"file":"host.ts","goImport":"github.com/oisee/abapiti/extern/hostio","goDir":%q}]}`, host),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"--tsconfig", filepath.Join(dir, "tsconfig.json"), "--externs", filepath.Join(dir, "externs.json"), "--entry", "probe.ts.Probe.bad", "--initialize", filepath.Join(dir, "initialization.json"), "-o", filepath.Join(dir, "go"), "probe.ts", "host.ts"}
	strict := newEmitGoCmd()
	strict.SilenceErrors, strict.SilenceUsage = true, true
	strict.SetArgs(args)
	if err := strict.Execute(); err == nil {
		t.Fatal("default emission accepted unsupported eval")
	}
	emit := newEmitGoCmd()
	emit.SetArgs(append([]string{"--trap-on-failure", "--overrides", "registry", "--build"}, args...))
	if err := emit.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"evidence/registry-traps.json", "evidence/registry-verify.txt"} {
		if _, err := os.Stat(filepath.Join(dir, "go", name)); err != nil {
			t.Fatal(err)
		}
	}
	sites, err := os.ReadFile(filepath.Join(dir, "go", "evidence", "trap-sites.json"))
	if err != nil || !strings.Contains(string(sites), `"Method": "bad"`) || !strings.Contains(string(sites), `"Source": "probe.ts:1:`) || !strings.Contains(string(sites), `"Kind": "body"`) {
		t.Fatalf("trap-sites.json lacks the located body trap: %v\n%s", err, sites)
	}
	program := exec.Command(filepath.Join(dir, "go", "program"))
	output, err := program.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "probe.ts:1:") || !strings.Contains(string(output), "lowering trap:") {
		t.Fatalf("wanted source-located runtime trap, got %v\n%s", err, output)
	}
	if text, err := os.ReadFile(marker); err != nil || string(text) != "initialized" {
		t.Fatalf("top-level host IO did not run before the trap: %q, %v", text, err)
	}
}
