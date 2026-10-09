package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
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
	// The deployment harness (a copy placed in the closure next to src/)
	// joins the lowered files when the run driver is requested.
	if h := os.Getenv("REGISTRY_HARNESS"); h != "" {
		files = append(files, h)
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
	entries = append(entries, overrides.RegistryDeployment()...)
	entries = append(entries, overrides.Syntax()...)
	registry, err := overrides.New(entries...)
	if err != nil {
		t.Fatal(err)
	}
	var coverage *Reachability
	if path := os.Getenv("REGISTRY_REACHABILITY"); path != "" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		coverage = &Reachability{CurrentInputs: &CoverageInputs{InputDir: os.Getenv("REGISTRY_INPUT"), DependenciesDir: os.Getenv("REGISTRY_DEPENDENCIES"), ConfigPath: os.Getenv("REGISTRY_CONFIG"), NegativesPath: os.Getenv("REGISTRY_NEGATIVES")}}
		if err := json.Unmarshal(data, coverage); err != nil {
			t.Fatal(err)
		}
		if coverage.UpstreamPin != manifest.Pin {
			t.Fatal("coverage upstream pin mismatch")
		}
	}
	prog, diags, err := p.LowerWithReachability(files, registry, coverage)
	if err != nil {
		t.Fatal(err)
	}
	var blocking []LowerDiagnostic
	for _, d := range diags {
		if d.Category == "note-declaration-reachability" {
			t.Log(d.Message)
		}
		if !strings.HasPrefix(d.Category, "note-") {
			blocking = append(blocking, d)
		}
	}
	verification := hir.Verify(prog)
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		var inventory []struct {
			ID, File, Symbol, Kind, SHA256, Rationale string
		}
		for _, e := range registry.Inventory() {
			inventory = append(inventory, struct{ ID, File, Symbol, Kind, SHA256, Rationale string }{e.ID, e.Key.File, e.Key.Symbol, e.Key.Kind, e.SHA256, e.Rationale})
		}
		data, err := json.MarshalIndent(inventory, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "registry-overrides.json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		data, err = json.MarshalIndent(blocking, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "registry-blocking.json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		var trapEvidence []struct{ Class, Method, Location string }
		for _, c := range prog.Classes {
			for _, m := range c.Methods {
				if m.Body != nil && m.Body.Kind == hir.Block && len(m.Body.List) == 1 && m.Body.List[0].Kind == hir.Trap {
					trapEvidence = append(trapEvidence, struct{ Class, Method, Location string }{c.Name, m.Name, m.Body.List[0].Name})
				}
			}
		}
		traps, err := json.MarshalIndent(trapEvidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "registry-traps.json"), append(traps, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, v := range verification {
			lines = append(lines, v.Error())
		}
		if err := os.WriteFile(filepath.Join(out, "registry-verify.txt"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("ABAPITI_TEST_DUMP") != "" {
			if err := os.WriteFile(filepath.Join(out, "registry-hir.txt"), []byte(hir.Dump(prog)), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("%d files, %d classes, %d interfaces, %d blocking diagnostics, %d HIR errors", len(files), len(prog.Classes), len(prog.Interfaces), len(blocking), len(verification))
	if len(blocking) != 0 || len(verification) != 0 {
		t.Fatalf("Registry closure is not translatable; do not emit partial output")
	}
	// Passing lowering is not acceptance by itself: exercise the actual ABAP
	// backend and preserve the complete emission for lint and differential work.
	emitted, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	if len(emitted) == 0 {
		t.Fatal("Registry closure emitted no files")
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		for name, contents := range emitted {
			if err := os.WriteFile(filepath.Join(out, name), []byte(contents), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if want := os.Getenv("REGISTRY_RUN_SHA"); want != "" {
		out := os.Getenv("ABAPITI_TEST_OUT")
		var inputs []RegistryFile
		for _, root := range []struct {
			dir string
			dep bool
		}{{envOr("REGISTRY_RUN_INPUT", "REGISTRY_INPUT"), false}, {envOr("REGISTRY_RUN_DEPENDENCIES", "REGISTRY_DEPENDENCIES"), true}} {
			var names []string
			if err := filepath.WalkDir(root.dir, func(path string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					names = append(names, path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			sort.Strings(names)
			for _, path := range names {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				rel, _ := filepath.Rel(root.dir, path)
				inputs = append(inputs, RegistryFile{Name: filepath.ToSlash(rel), Raw: string(data), Dependency: root.dep})
			}
		}
		config, err := os.ReadFile(os.Getenv("REGISTRY_CONFIG"))
		if err != nil {
			t.Fatal(err)
		}
		class := "zcl_abapiti_registry_run"
		if err := os.WriteFile(filepath.Join(out, class+".clas.abap"), []byte(RegistryRunClass(class, inputs, string(config), want, 0, names)), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, class+".clas.testclasses.abap"), []byte(RegistryRunTest(class, names.Get("exception.unexecuted"))), 0644); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("REGISTRY_RUN_CORPUS") != "" {
			a4h := "zcl_abapiti_registry_a4h"
			if err := os.WriteFile(filepath.Join(out, a4h+".clas.abap"), []byte(RegistryRunCorpusClass(a4h, want, names)), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, "zabapiti_registry_run.prog.abap"), []byte(RegistryRunReport("zabapiti_registry_run", a4h)), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("Registry closure emitted %d ABAP files", len(emitted))
}

// envOr reads the first variable, falling back to the second: the run
// driver may take other inputs than the coverage workload.
func envOr(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}
