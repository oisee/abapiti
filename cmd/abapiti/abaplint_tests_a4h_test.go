package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitTestPackages(t *testing.T) {
	files := map[string]string{
		"names.json":             `{"ZCL_LNT_CORE":{"id":"src/core.ts.Core"},"ZCL_LNT_TEST":{"id":"test/test.ts.module"},"ZLNT_HELPER":{"id":"runtime.array<string>"}}`,
		"zcl_lnt_core.clas.abap": "CLASS zcl_lnt_core. DATA x TYPE REF TO zlnt_helper. DATA(text) = `zcl_lnt_test`. ENDCLASS.",
		"zcl_lnt_test.clas.abap": "CLASS zcl_lnt_test. DATA x TYPE REF TO zcl_lnt_core. ENDCLASS.",
		"zlnt_helper.clas.abap":  "CLASS zlnt_helper. ENDCLASS.",
	}
	core, tests, err := splitTestPackages(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(core) != 2 || len(tests) != 1 {
		t.Fatalf("partition: %v / %v", core, tests)
	}
	files["zcl_lnt_core.clas.abap"] += " DATA bad TYPE REF TO zcl_lnt_test."
	if _, _, err := splitTestPackages(files); err == nil {
		t.Fatal("accepted reverse test dependency")
	}
}

func TestA4HSourceEnvelope(t *testing.T) {
	valid := map[string]string{"zcl_lnt_a.clas.abap": "CLASS zcl_lnt_a. ENDCLASS."}
	if err := validateA4HSources(valid); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []map[string]string{
		{"zcl_lnt_" + strings.Repeat("a", 30) + ".clas.abap": ""},
		{"zcl_lnt_b.clas.abap": strings.Repeat("x", 256)},
		{"zcl_lnt_BAD.clas.abap": ""},
	} {
		if err := validateA4HSources(invalid); err == nil {
			t.Fatal("accepted invalid envelope")
		}
	}
	if err := validateA4HSources(valid, valid); err == nil {
		t.Fatal("accepted cross-package collision")
	}
}

func TestAbapGitObjectManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zip")
	sources := map[string]string{"zcl_lnt_a.clas.abap": "CLASS zcl_lnt_a. ENDCLASS.", "zlnt_testrun.tabl.xml": "<table/>"}
	n, err := writeAbapGitZip(path, "$ZLNT", "tests", sources)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatal(n)
	}
	if err := writeAbapGitManifest(path, "$ZLNT"); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "test.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Objects map[string]struct {
			SHA256 string
			Files  []string
		}
	}
	if err := json.Unmarshal(first, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Objects) != 3 {
		t.Fatal("manifest must include DEVC")
	}
	for name, object := range manifest.Objects {
		if len(object.SHA256) != 64 || len(object.Files) == 0 {
			t.Fatal(name, object)
		}
	}
	if err := writeAbapGitManifest(path, "$ZLNT"); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "test.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("non-reproducible manifest")
	}
	if strings.Contains(string(first), dir) {
		t.Fatal("absolute path in manifest")
	}
}
