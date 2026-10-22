package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// Exercise real cached syntax and stored macro expansions against fresh Checkers.
// Opt in because the full emitted core is expensive to compile.
func TestRegistryGoLibraryTransitive(t *testing.T) {
	library := os.Getenv("ABAPITI_LIBRARY_TRANSITIVE_DIR")
	if library == "" {
		t.Skip("set ABAPITI_LIBRARY_TRANSITIVE_DIR to the emitted full-core Go module")
	}
	library, err := filepath.Abs(library)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, pair := range [][2]string{{"transitive_test.go.txt", "transitive_test.go"}, {"transitive.json", "transitive.json"}} {
		raw, err := os.ReadFile(filepath.Join("testdata", "library-incremental", pair[0]))
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
	cmd := exec.Command("go", "test", "-p=1", "-gcflags=-N", "-count=1", "-v", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("full-core transitive fixtures: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
