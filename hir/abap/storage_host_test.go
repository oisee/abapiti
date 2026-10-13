package abap

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Always exercise formatting; opt into the pinned gogen compiler and kernel
// with ABAPITI_OSG_HOME. ABAPITI_TEST_OUT retains the exact executed sources.
func storageHostFixture(t *testing.T, files map[string]string, owner string) {
	t.Helper()
	dir := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		dir = filepath.Join(base, t.Name())
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, source := range files {
		formatted, err := Format(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(formatted), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if osg := os.Getenv("ABAPITI_OSG_HOME"); osg != "" {
		cmd := exec.Command("node", filepath.Join(osg, "tools/gogen/unit.mjs"), "--input", dir, "--class", owner, "--jobs", "1", "--no-cache", "--out", t.TempDir())
		cmd.Env = append(os.Environ(), "OSG_HOME="+osg, "GOFLAGS=-p=2")
		out, err := cmd.Output()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				t.Fatalf("OSG: %v\n%s\n%s", err, out, exit.Stderr)
			}
			t.Fatal(err)
		}
		var report struct {
			Compiled int `json:"compiled"`
			Rows     []struct {
				Status string `json:"status"`
			} `json:"rows"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("OSG JSON: %v\n%s", err, out)
		}
		if report.Compiled != 1 || len(report.Rows) == 0 {
			t.Fatalf("OSG did not execute fixture: %s", out)
		}
		for _, row := range report.Rows {
			if row.Status != "SUCCESS" {
				t.Fatalf("OSG fixture: %s", out)
			}
		}
		t.Logf("OSG: %d SUCCESS rows", len(report.Rows))
	}
}
