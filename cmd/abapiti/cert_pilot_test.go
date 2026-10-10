package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	gohir "github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/tsfront"
)

// TestGoCertificatePilot compiles source-pinned original bodies required by the
// guarded graph, and executes the pre-store tests in fresh processes. Acceptance
// is tested separately: proposed/revoked certificates cannot reach CLI emission.
func TestGoCertificatePilot(t *testing.T) {
	if os.Getenv("ABAPITI_CERT_PILOT_TEST") == "" {
		t.Skip("set ABAPITI_CERT_PILOT_TEST=1 for full Go pilot gate")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	src, err := embeddedAbaplint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	c := tsfront.EmbeddedRegistryClosure()
	if err := materializeClosure(src, c, work); err != nil {
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
	l, err := tsfront.LowerRegistry(work, append(c.Files(), tsfront.RegistryHarnessPath), registry, tsfront.PilotWarmupCoverage(coverage))
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Err(); err != nil {
		t.Fatal(err)
	}
	files, err := gohir.Emit(l.Prog)
	if err != nil {
		t.Fatal(err)
	}
	if vanilla := os.Getenv("ABAPITI_CERT_PILOT_VANILLA_OUT"); vanilla != "" {
		original := map[string]string{}
		for name, value := range files {
			original[name] = value
		}
		original["main.go"] = tsfront.RegistryGoCLI()
		original["go.mod"] = "module vanilla\n\ngo 1.26.0\n"
		if err := writeSources(vanilla, original); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(vanilla, "zabaplint"), ".")
		cmd.Dir = vanilla
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("vanilla: %v\n%s", err, out)
		}
	}
	if err := tsfront.RegistryGoCertificatePilot(files); err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = "module pilot\n\ngo 1.26.0\n"
	dir := t.TempDir()
	if output := os.Getenv("ABAPITI_CERT_PILOT_OUT"); output != "" {
		dir = output
	}
	if err := writeSources(dir, files); err != nil {
		t.Fatal(err)
	}
	for _, pattern := range []string{"TestPilotPrestore|TestPilotSnapshot", "TestPilotVanilla"} {
		cmd := exec.Command("go", "test", "-v", "-run", pattern)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false", "ABAPITI_PILOT_VANILLA_TEST=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", pattern, err, out)
		} else {
			t.Log(string(out))
		}
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "zabaplint"), ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pilot build: %v\n%s", err, out)
	}
}
