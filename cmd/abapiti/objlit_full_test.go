package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/tsfront"
)

// Opt-in diagnostic probe: all pinned files, no reachability pruning, binary64.
func TestObjectLiteralFullClosure(t *testing.T) {
	out := os.Getenv("OBJLIT_FULL_OUT")
	if out == "" {
		t.Skip("set OBJLIT_FULL_OUT to save unpruned lowering evidence")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "")
	src, err := embeddedAbaplint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	closure := tsfront.EmbeddedRegistryClosure()
	if err := materializeClosure(src, closure, work); err != nil {
		t.Fatal(err)
	}
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	lowering, err := tsfront.LowerRegistry(work, append(closure.Files(), tsfront.RegistryHarnessPath), registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := lowering.Evidence(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range evidence {
		if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d closure files: %d blocking, %d HIR errors", len(closure.Sources), len(lowering.Blocking), len(lowering.Verification))
}
