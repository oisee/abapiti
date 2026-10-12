package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
)

func TestLowerFullProject(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"custom.json":   `{"compilerOptions":{"lib":["es2020"],"noEmit":true},"include":["src/*.ts","core/*.ts"]}`,
		"src/probe.ts":  `export class Probe { static run(): void { console.log("x"); } }`,
		"core/probe.ts": `export class Core { static run(): void { console.log("x"); } }`,
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "out")
	cmd := newLowerCmd()
	cmd.SetArgs([]string{"--full", "--tsconfig", filepath.Join(dir, "custom.json"), "--scope", "src", "-o", out, "src/probe.ts", "core/probe.ts"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	readSummary := func(name string) gapSummary {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, "evidence", name))
		if err != nil {
			t.Fatal(err)
		}
		var s gapSummary
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	scoped, all := readSummary("gap-summary.json"), readSummary("closure-gap-summary.json")
	if scoped.Blocking == 0 || all.Blocking != 2*scoped.Blocking || scoped.Files != 1 || scoped.ByRootMessage[0].Example == "" {
		t.Fatalf("scope %+v, all %+v", scoped, all)
	}
	data, err := os.ReadFile(filepath.Join(out, "evidence", "checker-diagnostics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checker []tsfront.Diagnostic
	if err := json.Unmarshal(data, &checker); err != nil {
		t.Fatal(err)
	}
	if len(checker) != 1 || !sourceInScope(checker[0].Loc, filepath.Join(dir, "src")) {
		t.Fatal(checker)
	}
	// Measurement reports gaps successfully; it must not emit target code.
	if _, err := os.Stat(filepath.Join(out, "classes")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestLowerScopeBoundaries(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "src")
	inside := filepath.Join(dir, "probe.ts") + ":1:2"
	outside := filepath.Join(dir+"-other", "probe.ts") + ":1:2"
	r := &tsfront.RegistryLowering{
		Blocking:     []tsfront.LowerDiagnostic{{Loc: inside}, {Loc: outside}},
		Verification: []error{hir.Error{Node: hir.Node{Source: inside}}, hir.Error{Node: hir.Node{Source: outside}}, hir.Error{Node: hir.Node{Source: "synthetic"}}},
	}
	s := scopedLowering(r, dir)
	if len(s.Blocking) != 1 || len(s.Verification) != 1 || len(r.Blocking) != 2 {
		t.Fatal(s)
	}
}
