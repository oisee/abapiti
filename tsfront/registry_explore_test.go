package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// This is a strict diagnostic gate, not a passing Registry implementation.
// tools/registry-closure.mjs supplies byte-pinned original sources. Parser-only
// Registry/INCLUDE exclusions are removed so they cannot hide reachable code.
func TestRegistryClosureGate(t *testing.T) {
	dir := os.Getenv("REGISTRY_CLOSURE")
	if dir == "" {
		t.Skip("set REGISTRY_CLOSURE to a materialized pinned upstream closure")
	}
	var manifest struct {
		Pin     string `json:"upstreamPin"`
		Sources []struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"sources"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Pin != "577f875ebec44cfaf64841cfe71c8ab8dc32622e" || len(manifest.Sources) == 0 {
		t.Fatal("Registry closure must name the original upstream pin and contain sources")
	}
	var files []string
	for _, source := range manifest.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, source.File))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != source.SHA256 {
			t.Fatalf("changed pinned source: %s", source.File)
		}
		files = append(files, source.File)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []overrides.Entry
	for _, e := range overrides.Abaplint().Inventory() {
		switch e.ID {
		case "abaplint-registry-scope", "abaplint-external-include", "abaplint-registry-input-progress":
			continue
		}
		entries = append(entries, e)
	}
	registry, err := overrides.New(entries...)
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithOverrides(files, registry)
	if err != nil {
		t.Fatal(err)
	}
	var blocking []LowerDiagnostic
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			blocking = append(blocking, d)
		}
	}
	verification := hir.Verify(prog)
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(blocking, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "registry-blocking.json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, v := range verification {
			lines = append(lines, v.Error())
		}
		if err := os.WriteFile(filepath.Join(out, "registry-verify.txt"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d files, %d classes, %d interfaces, %d blocking diagnostics, %d HIR errors", len(files), len(prog.Classes), len(prog.Interfaces), len(blocking), len(verification))
	if len(blocking) != 0 || len(verification) != 0 {
		t.Fatalf("Registry closure is not translatable; do not emit partial output")
	}
}
