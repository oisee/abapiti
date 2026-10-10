package main

import (
	gohir "github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/tsfront"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Same source materialization, overrides, reachability and assume-int default
// as the production CLI. This opt-in gate compiles every emitted class.
func TestGoFullClosure(t *testing.T) {
	if os.Getenv("ABAPITI_GO_FULL_TEST") == "" {
		t.Skip("set ABAPITI_GO_FULL_TEST=1 for the full closure compilation gate")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	src, err := embeddedAbaplint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	c := tsfront.EmbeddedRegistryClosure()
	if err = materializeClosure(src, c, work); err != nil {
		t.Fatal(err)
	}
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := tsfront.EmbeddedReachability()
	if err != nil {
		t.Fatal(err)
	}
	l, err := tsfront.LowerRegistry(work, append(c.Files(), tsfront.RegistryHarnessPath), registry, coverage)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Err(); err != nil {
		t.Fatal(err)
	}
	if len(l.Prog.Classes) != 1927 || len(l.Prog.Interfaces) != 73 {
		t.Fatalf("changed closure: %d classes, %d interfaces", len(l.Prog.Classes), len(l.Prog.Interfaces))
	}
	files, err := gohir.Emit(l.Prog)
	if err != nil {
		t.Fatal(err)
	}
	files["main.go"] = tsfront.RegistryGoCLI()
	files["go.mod"] = "module full\n\ngo 1.26.0\n"
	dir := t.TempDir()
	if err = writeSources(dir, files); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "zabaplint"), ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("full closure compilation: %v\n%s", err, out)
	}
	t.Log("1538 pinned files, 1927 classes, 73 interfaces: complete Go executable compiled")
}
