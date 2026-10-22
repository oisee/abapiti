package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// The full pinned parser fixture gate is opt-in because compiling 2,079 emitted
// classes is too expensive for every short run. The lifecycle test double still
// compiles and exercises lifecycle/concat semantics in the ordinary short suite.
func TestRegistryGoLibraryTree(t *testing.T) {
	library := os.Getenv("ABAPITI_LIBRARY_TREE_DIR")
	if library == "" {
		t.Skip("set ABAPITI_LIBRARY_TREE_DIR to the emitted full-core Go module")
	}
	library, err := filepath.Abs(library)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, pair := range [][2]string{{"api_test.go.txt", "api_test.go"}, {"files.json", "files.json"}} {
		raw, err := os.ReadFile(filepath.Join("testdata", "library-tree", pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, pair[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	mod := "module librarytreefixture\n\ngo 1.26.0\n\nrequire zabaplint v0.0.0\nreplace zabaplint => " + strconv.Quote(library) + "\n"
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("full-core Tree fixtures: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
