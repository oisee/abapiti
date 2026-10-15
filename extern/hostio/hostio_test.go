package hostio

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNodePaths(t *testing.T) {
	// Expected values recorded from Node path.posix.
	for _, c := range []struct{ s, base, dir string }{
		{"", "", "."}, {".", ".", "."}, {"..", "..", "."}, {"a/", "a", "."}, {"/a//", "a", "/"}, {"//", "", "/"}, {"///foo//bar///", "bar", "///foo/"}, {"a//b/..", "..", "a//b"}, {"/", "", "/"}, {"a/b", "b", "a"}, {"//foo", "foo", "//"},
	} {
		if b, d := Basename(c.s), Dirname(c.s); b != c.base || d != c.dir {
			t.Errorf("%q: %q %q; want %q %q", c.s, b, d, c.base, c.dir)
		}
	}
	for _, c := range []struct {
		parts []string
		want  string
	}{
		{nil, "."}, {[]string{"", ""}, "."}, {[]string{".", ".."}, ".."}, {[]string{"a", "/b"}, "a/b"}, {[]string{"//", "foo/", "..", "bar/"}, "/bar/"}, {[]string{"a", "..", ""}, "."}, {[]string{"/", ".."}, "/"},
	} {
		if got := Join(c.parts...); got != c.want {
			t.Errorf("join %q: %q != %q", c.parts, got, c.want)
		}
	}
}
func TestReadTextNodeUTF8(t *testing.T) {
	f := filepath.Join(t.TempDir(), "text")
	for _, c := range []struct{ hex, want string }{{"ff", "�"}, {"c328", "�("}, {"eda080", "���"}, {"e282", "�"}, {"f09f", "�"}, {"f09f9880", "😀"}, {"e18041", "�A"}} {
		b, _ := hex.DecodeString(c.hex)
		if err := os.WriteFile(f, b, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := ReadText(f)
		if err != nil || got != c.want {
			t.Errorf("%s: %q %v, want %q", c.hex, got, err, c.want)
		}
	}
}
func TestFiles(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "z")
	if err := WriteText(f, "old"); err != nil {
		t.Fatal(err)
	}
	if err := WriteText(f, "😀"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f, filepath.Join(dir, "c")); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadText(filepath.Join(dir, "c")); err != nil || got != "😀" {
		t.Fatal(got, err)
	}
	if got, err := ListDir(dir); err != nil || !reflect.DeepEqual(got, []string{"a", "b", "c", "z"}) {
		t.Fatal(got, err)
	}
	es, err := ListEntries(dir)
	if err != nil || len(es) != 4 || !es[0].F1 || es[1].F1 {
		t.Fatal(es, err)
	}
	if err := WriteText(filepath.Join(dir, "missing", "file"), "x"); err == nil {
		t.Fatal("created parent")
	}
	if _, err := ReadText(dir + "/missing"); err == nil {
		t.Fatal("missing read")
	}
	if _, err := ListDir(f); err == nil {
		t.Fatal("list file")
	}
	if _, err := ListEntries(f); err == nil {
		t.Fatal("entries file")
	}
	t.Setenv("ABAPITI_HOSTIO_TEST", "")
	if v, ok := Env("ABAPITI_HOSTIO_TEST"); !ok || v != "" {
		t.Fatal(v, ok)
	}
	t.Setenv("ABAPITI_HOSTIO_TEST", "value")
	if v, ok := Env("ABAPITI_HOSTIO_TEST"); !ok || v != "value" {
		t.Fatal(v, ok)
	}
	if _, ok := Env("ABAPITI_HOSTIO_TEST_MISSING_1F32"); ok {
		t.Fatal("unexpected env")
	}
	if _, ok := GitHead(dir); ok {
		t.Fatal("unexpected git head")
	}
}
func TestGitHead(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git absent")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-q", "--allow-empty", "-m", "test"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = dir
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, b)
		}
	}
	if v, ok := GitHead(dir); !ok || len(v) != 40 {
		t.Fatal(v, ok)
	}
}

func TestDirectoryNamesNodeUTF8(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node absent")
	}
	for _, nameHex := range []string{"e282", "f09f", "eda080", "ff", "e18041", "e08080", "f4908080", "f09f9880"} {
		t.Run(nameHex, func(t *testing.T) {
			dir := t.TempDir()
			name, err := hex.DecodeString(nameHex)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, string(name)), nil, 0600); err != nil {
				t.Skipf("filesystem refuses filename %x: %v", name, err)
			}
			cmd := exec.Command("nice", node, "-e", `const fs=require('node:fs');const d=process.argv[1];console.log(JSON.stringify([fs.readdirSync(d),fs.readdirSync(d,{withFileTypes:true}).map(e=>[e.name,e.isDirectory()])]));`, dir)
			b, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Node: %v %s", err, b)
			}
			var want []json.RawMessage
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			names, err := ListDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			var wantNames []string
			if err := json.Unmarshal(want[0], &wantNames); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(names, wantNames) {
				t.Errorf("ListDir: %q, Node: %q", names, wantNames)
			}
			entries, err := ListEntries(dir)
			if err != nil {
				t.Fatal(err)
			}
			// Compare before JSON encoding, which itself replaces malformed UTF-8.
			var wantEntries [][2]json.RawMessage
			if err := json.Unmarshal(want[1], &wantEntries); err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(wantEntries) {
				t.Fatalf("ListEntries length: %d, Node: %d", len(entries), len(wantEntries))
			}
			for i, e := range entries {
				var name string
				var isDir bool
				if err := json.Unmarshal(wantEntries[i][0], &name); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(wantEntries[i][1], &isDir); err != nil {
					t.Fatal(err)
				}
				if e.F0 != name || e.F1 != isDir {
					t.Errorf("ListEntries: %q/%v, Node: %q/%v", e.F0, e.F1, name, isDir)
				}
			}
		})
	}
}
