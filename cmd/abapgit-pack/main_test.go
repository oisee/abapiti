package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func writeSources(t *testing.T, sources map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func archiveFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	var paths []string
	for _, f := range r.File {
		paths = append(paths, f.Name)
		if !f.Modified.Equal(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("%s: unexpected timestamp %s", f.Name, f.Modified)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := files[f.Name]; exists {
			t.Fatalf("duplicate entry %s", f.Name)
		}
		files[f.Name] = string(data)
	}
	if !sort.StringsAreSorted(paths) {
		t.Errorf("ZIP entries are not sorted: %v", paths)
	}
	return files
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestObjectKinds(t *testing.T) {
	for _, tt := range []struct {
		name, filename, kind, golden string
		includes                     []string
	}{
		{"class", "zcl_demo.clas.abap", "clas", "class.xml", nil},
		{"class with includes", "zcl_demo.clas.abap", "clas", "class_tests.xml", []string{"testclasses", "locals_imp", "locals_def", "macros"}},
		{"class without tests", "zcl_demo.clas.abap", "clas", "class.xml", []string{"locals_imp", "locals_def", "macros"}},
		{"interface", "zif_demo.intf.abap", "intf", "interface.xml", nil},
		{"report", "zdemo.prog.abap", "prog", "report.xml", nil},
		{"legacy report", "zdemo.abap", "prog", "report.xml", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sources := map[string]string{tt.filename: "source\n"}
			for _, include := range tt.includes {
				sources["zcl_demo.clas."+include+".abap"] = include + "\n"
			}
			opts := options{dir: writeSources(t, sources), mask: "*.abap", desc: "Demo", pkg: "ZPACKAGE"}
			objects, err := readObjects(opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(objects) != 1 || objects[0].kind != tt.kind {
				t.Fatalf("unexpected objects: %+v", objects)
			}
			var buf bytes.Buffer
			if err := writeArchive(&buf, objects, opts.desc); err != nil {
				t.Fatal(err)
			}
			files := archiveFiles(t, buf.Bytes())
			if len(files) != len(sources)+3 {
				t.Errorf("unexpected entries: %v", files)
			}
			for name, source := range sources {
				if tt.name == "legacy report" {
					name = "zdemo.prog.abap"
				}
				if files["src/"+name] != source {
					t.Errorf("source not preserved: %s", name)
				}
			}
			base := strings.TrimSuffix(tt.filename, ".abap")
			if tt.name == "legacy report" {
				base += ".prog"
			}
			checkGolden(t, tt.golden, files["src/"+base+".xml"])
			checkGolden(t, "package.xml", files["src/package.devc.xml"])
			checkGolden(t, "abapgit.xml", files[".abapgit.xml"])
		})
	}
}

func TestDeterminism(t *testing.T) {
	sources := map[string]string{
		"zcl_demo.clas.abap":             "class\n",
		"zcl_demo.clas.testclasses.abap": "tests\n",
		"zif_demo.intf.abap":             "interface\n",
		"zdemo.prog.abap":                "report\n",
	}
	dir := writeSources(t, sources)
	opts := options{dir: dir, mask: "*.abap", desc: "Demo", pkg: "$TMP"}
	first, second := filepath.Join(t.TempDir(), "first.zip"), filepath.Join(t.TempDir(), "second.zip")
	if err := run(opts, first); err != nil {
		t.Fatal(err)
	}
	// Changes in source mtimes must not change the archive.
	for name := range sources {
		stamp := time.Date(2025, time.July, 2, 3, 4, 5, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(dir, name), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := run(opts, second); err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("identical inputs produced different ZIP bytes")
	}
	files := archiveFiles(t, a)
	for _, name := range []string{"zcl_demo.clas.xml", "zif_demo.intf.xml", "zdemo.prog.xml"} {
		if _, exists := files["src/"+name]; !exists {
			t.Errorf("missing %s", name)
		}
	}
}

func TestInvalidNames(t *testing.T) {
	for _, name := range []string{"", strings.Repeat("z", 31), "z-bad", "z bad", "z&bad", "z.bad", "1bad", "zé", "z#bad", "../zbad"} {
		t.Run(name, func(t *testing.T) {
			dir := writeSources(t, map[string]string{"input.abap": "REPORT input."})
			file := filepath.Join(dir, "input.abap")
			if name == "" {
				file = filepath.Join(dir, ".prog.abap")
				if err := os.WriteFile(file, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := readObjects(options{file: file, name: name})
			if err == nil || !strings.Contains(err.Error(), "invalid object name") {
				t.Fatalf("expected clear name error, got %v", err)
			}
		})
	}
	for _, filename := range []string{strings.Repeat("z", 31) + ".clas.abap", "z-bad.intf.abap", "z bad.prog.abap"} {
		t.Run(filename, func(t *testing.T) {
			dir := writeSources(t, map[string]string{filename: "source"})
			_, err := readObjects(options{dir: dir, mask: "*.abap"})
			if err == nil || !strings.Contains(err.Error(), "invalid object name") {
				t.Fatalf("expected clear name error, got %v", err)
			}
		})
	}
}

func TestSingleReport(t *testing.T) {
	dir := writeSources(t, map[string]string{"input.abap": "REPORT zdemo.\n"})
	for _, name := range []string{"zdemo", "ZDEMO"} {
		objects, err := readObjects(options{file: filepath.Join(dir, "input.abap"), name: name})
		if err != nil {
			t.Fatal(err)
		}
		want := []object{{name: "ZDEMO", kind: "prog", sources: map[string]string{"src/zdemo.prog.abap": "REPORT zdemo.\n"}}}
		if !reflect.DeepEqual(objects, want) {
			t.Fatalf("got %+v, want %+v", objects, want)
		}
		checkGolden(t, "report.xml", objectXML(objects[0], "Demo"))
	}
	// Preserve the original report title length cap.
	obj := object{name: "ZDEMO", kind: "prog"}
	if !strings.Contains(objectXML(obj, strings.Repeat("a", 80)), "<LENGTH>70</LENGTH>") {
		t.Fatal("report title length cap changed")
	}
}

func TestSourceErrors(t *testing.T) {
	for _, tt := range []struct {
		name, message string
		sources       map[string]string
	}{
		{"orphan include", "requires main source", map[string]string{"zcl_demo.clas.testclasses.abap": "tests"}},
		{"unknown include", "unsupported ABAP source filename", map[string]string{"zcl_demo.clas.unknown.abap": "unknown"}},
		{"duplicate", "duplicate source", map[string]string{"zdemo.prog.abap": "report", "ZDEMO.prog.abap": "report"}},
		{"empty", "no files found", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readObjects(options{dir: writeSources(t, tt.sources), mask: "*.abap"})
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("expected %q, got %v", tt.message, err)
			}
		})
	}
}

func TestNamesAndEscaping(t *testing.T) {
	for _, name := range []string{strings.Repeat("z", 30), "ZCL_DEMO", "#demo#zcl_test"} {
		dir := writeSources(t, map[string]string{name + ".clas.abap": "source"})
		objects, err := readObjects(options{dir: dir, mask: "*.abap"})
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := writeArchive(&buf, objects, `A & B <demo>`); err != nil {
			t.Fatal(err)
		}
		files := archiveFiles(t, buf.Bytes())
		for path, content := range files {
			if !strings.HasSuffix(path, ".xml") {
				continue
			}
			decoder := xml.NewDecoder(strings.NewReader(content))
			for {
				_, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("%s: invalid XML: %v", path, err)
				}
			}
		}
		metadata := files["src/"+strings.ToLower(name)+".clas.xml"]
		if !strings.Contains(metadata, "<CLSNAME>"+strings.ToUpper(strings.ReplaceAll(name, "#", "/"))+"</CLSNAME>") {
			t.Fatalf("incorrect CLSNAME: %s", metadata)
		}
		if !strings.Contains(metadata, "A &amp; B &lt;demo&gt;") {
			t.Fatalf("description not escaped: %s", metadata)
		}
	}
}

func TestWriteError(t *testing.T) {
	if err := writeArchive(failingWriter{}, nil, "Demo"); err == nil {
		t.Fatal("expected archive write error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
