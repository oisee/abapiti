package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corpusabap "github.com/oisee/abapiti/tools/allbackends/corpus-abap"
	"github.com/oisee/abapiti/tsfront"
)

// The abaplint sources and npm type declarations built into abapiti unpack
// to a tree that verifies against the pinned closure and package manifests.
func TestEmbeddedAbaplint(t *testing.T) {
	src, err := embeddedAbaplint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !src.Embedded || src.Files != 1539 || len(src.Packages) != 3 {
		t.Fatalf("unpacked %d files, %d packages", src.Files, len(src.Packages))
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	strip := func(name string) (string, bool) {
		rest, ok := strings.CutPrefix(name, "package/")
		return rest, ok && rest != ""
	}
	dir := t.TempDir()
	n, err := extractTarGz(bytes.NewReader(tarGz(t, map[string]string{"package/lib/a.d.ts": "x", "other/b": "y"})), dir, strip)
	if err != nil || n != 1 {
		t.Fatalf("extracted %d files: %v", n, err)
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "lib", "a.d.ts")); err != nil || string(raw) != "x" {
		t.Fatalf("lib/a.d.ts: %q %v", raw, err)
	}
	if _, err := extractTarGz(bytes.NewReader(tarGz(t, map[string]string{"package/../evil": "x"})), t.TempDir(), strip); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestWriteAbapGitZip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.zip")
	objects, err := writeAbapGitZip(file, "$ZABAPLINT", "test", map[string]string{
		"zcl_a.clas.abap":             "CLASS zcl_a DEFINITION.",
		"zcl_embedded.clas.abap":      "CLASS zcl_embedded DEFINITION.",
		"zcl_embedded.clas.xml":       "<preserved-class-metadata/>",
		"zcl_a.clas.testclasses.abap": "CLASS ltcl DEFINITION FOR TESTING.",
		"zif_b.intf.abap":             "INTERFACE zif_b.",
		"zprog.prog.abap":             "REPORT zprog.",
	})
	if err != nil || objects != 4 {
		t.Fatalf("objects %d: %v", objects, err)
	}
	r, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		rc.Close()
		got[f.Name] = b.String()
	}
	for _, name := range []string{".abapgit.xml", "src/package.devc.xml", "src/zcl_a.clas.abap", "src/zcl_a.clas.xml", "src/zcl_a.clas.testclasses.abap", "src/zif_b.intf.xml", "src/zprog.prog.xml"} {
		if _, ok := got[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	if !strings.Contains(got["src/zcl_a.clas.xml"], "<WITH_UNIT_TESTS>X</WITH_UNIT_TESTS>") || got["src/zcl_embedded.clas.xml"] != "<preserved-class-metadata/>" || !strings.Contains(got["src/zif_b.intf.xml"], "<CLSNAME>ZIF_B</CLSNAME>") {
		t.Error("class or interface metadata wrong")
	}
	if _, err := writeAbapGitZip(file, "$Z", "t", map[string]string{"x.txt": ""}); err == nil {
		t.Error("unsupported file accepted")
	}
}

func TestParseTargets(t *testing.T) {
	got, err := parseTargets("all")
	if err != nil || !got["a4h"] || !got["osg"] || !got["native"] || got["go"] {
		t.Fatalf("all: %v %v", got, err)
	}
	if got, err := parseTargets("native, osg"); err != nil || got["a4h"] || !got["native"] || !got["osg"] {
		t.Fatalf("native,osg: %v %v", got, err)
	}
	if got, err := parseTargets("abap"); err != nil || !got["a4h"] || !got["osg"] || !got["native"] || got["go"] {
		t.Fatalf("abap: %v %v", got, err)
	}
	if got, err := parseTargets("go"); err != nil || len(got) != 1 || !got["go"] {
		t.Fatalf("go: %v %v", got, err)
	}
	if _, err := parseTargets("wasm"); err == nil {
		t.Fatal("unknown target accepted")
	}
}

func TestAbaplintFullCoreDefaults(t *testing.T) {
	for flag, want := range map[string]string{"scope": "core", "assume-int": "false", "package": "$ZLNT"} {
		if got := abaplintCmd.Flags().Lookup(flag).DefValue; got != want {
			t.Fatalf("--%s default %q, want %q", flag, got, want)
		}
	}
}

// TestAbaplintCommand runs the whole command on a pinned checkout
// (ABAPITI_ABAPLINT_CHECKOUT, e.g. a clone at 577f875e after npm ci).
func TestAbaplintCommand(t *testing.T) {
	checkout := os.Getenv("ABAPITI_ABAPLINT_CHECKOUT")
	if checkout == "" {
		t.Skip("set ABAPITI_ABAPLINT_CHECKOUT to an abaplint checkout at 577f875e")
	}
	t.Setenv("ABAPITI_NAMES", "readable")
	out := t.TempDir()
	negative := filepath.Join(out, "negative.json")
	if err := os.WriteFile(negative, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{"abaplint", checkout, "-o", out, "-q", "--negative", negative})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	classes, err := os.ReadDir(filepath.Join(out, "classes"))
	if err != nil {
		t.Fatal(err)
	}
	lib, _ := os.ReadDir(filepath.Join(out, "native", "lib"))
	if len(classes) < 2000 || len(lib) != len(classes) {
		t.Fatalf("classes %d, native/lib %d", len(classes), len(lib))
	}
	for _, f := range []string{"native/zabaplint.prog.abap", "a4h/abaplint-577f875e-a4h.zip", "a4h/abaplint-577f875e-a4h-registry.zip", "osg/README.txt"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Error(err)
		}
	}
	assertA4HArchiveSplit(t, out)
}

// Check every archive entry, including XML, for accidental runtime coupling.
func assertA4HArchiveSplit(t *testing.T, out string) {
	t.Helper()
	read := func(name string) map[string]string {
		t.Helper()
		r, err := zip.OpenReader(filepath.Join(out, "a4h", name))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := r.Close(); err != nil {
				t.Error(err)
			}
		}()
		files := map[string]string{}
		for _, f := range r.File {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(rc)
			closeErr := rc.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			files[f.Name] = string(data)
		}
		return files
	}
	d := tsfront.Drivers()
	core := read("abaplint-577f875e-a4h.zip")
	for name, body := range core {
		content := strings.ToLower(name + "\n" + body)
		for _, forbidden := range []string{d.A4HClass, d.RunReport, d.CleanReport, d.NegReport, "zcl_abapiti_"} {
			if strings.Contains(content, strings.ToLower(forbidden)) {
				t.Errorf("core %s references %s", name, forbidden)
			}
		}
	}
	registry := read("abaplint-577f875e-a4h-registry.zip")
	entries, err := corpusabap.Runtime.ReadDir("src")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		body, err := corpusabap.Runtime.ReadFile("src/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if registry["src/"+entry.Name()] != string(body) {
			t.Errorf("embedded runtime changed: %s", entry.Name())
		}
	}
	for _, object := range []string{d.A4HClass + ".clas", d.RunReport + ".prog", d.CleanReport + ".prog", d.NegReport + ".prog", "zcl_abapiti_corpus.clas", "zcl_abapiti_log.clas", "zcl_lnt_test_log_bal.clas", "zif_abapiti_corpus.intf", "zabapiti_corpus_load.prog", "zabapiti_log_setup.prog"} {
		for _, ext := range []string{".abap", ".xml"} {
			if registry["src/"+object+ext] == "" {
				t.Errorf("registry missing %s%s", object, ext)
			}
		}
	}
	for _, table := range []string{"zabapiti_corpus", "zabapiti_runlog"} {
		if registry["src/"+table+".tabl.xml"] == "" {
			t.Errorf("registry missing %s", table)
		}
	}
	if !strings.Contains(registry["src/package.devc.xml"], "$ZLNT_A4H") {
		t.Error("registry package missing")
	}
}
