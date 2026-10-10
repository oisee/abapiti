package tsfront

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// This is a strict diagnostic gate, not a passing Registry implementation.
// tools/registry-closure.mjs supplies byte-pinned original sources. Parser-only
// Registry/INCLUDE exclusions are removed so they cannot hide reachable code.
func TestRegistryClosureGate(t *testing.T) {
	dir := os.Getenv("REGISTRY_CLOSURE")
	if dir == "" {
		t.Skip("set REGISTRY_CLOSURE to a materialized pinned upstream closure")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ParseRegistryClosure(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Verify(dir); err != nil {
		t.Fatal(err)
	}
	files := manifest.Files()
	// The deployment harness (a copy placed in the closure next to src/)
	// joins the lowered files when the run driver is requested.
	if h := os.Getenv("REGISTRY_HARNESS"); h != "" {
		files = append(files, h)
	}
	registry, err := RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	var coverage *Reachability
	if path := os.Getenv("REGISTRY_REACHABILITY"); path != "" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		coverage, err = ParseReachability(data, &CoverageInputs{InputDir: os.Getenv("REGISTRY_INPUT"), DependenciesDir: os.Getenv("REGISTRY_DEPENDENCIES"), ConfigPath: os.Getenv("REGISTRY_CONFIG"), NegativesPath: os.Getenv("REGISTRY_NEGATIVES")})
		if err != nil {
			t.Fatal(err)
		}
	}
	lowering, err := LowerRegistry(dir, files, registry, coverage)
	if err != nil {
		t.Fatal(err)
	}
	for _, note := range lowering.Notes {
		t.Log(note)
	}
	for _, e := range lowering.TrappedBases {
		t.Error(e)
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		evidence, err := lowering.Evidence(os.Getenv("ABAPITI_TEST_DUMP") != "")
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range evidence {
			if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	prog := lowering.Prog
	t.Logf("%d files, %d classes, %d interfaces, %d blocking diagnostics, %d HIR errors", len(files), len(prog.Classes), len(prog.Interfaces), len(lowering.Blocking), len(lowering.Verification))
	if len(lowering.Blocking) != 0 || len(lowering.Verification) != 0 {
		t.Fatalf("Registry closure is not translatable; do not emit partial output")
	}
	// Passing lowering is not acceptance by itself: exercise the actual ABAP
	// backend and preserve the complete emission for lint and differential work.
	emitted, names, err := lowering.Emit()
	if err != nil {
		t.Fatal(err)
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
		inputs, err := ReadRegistryInputs(envOr("REGISTRY_RUN_INPUT", "REGISTRY_INPUT"), envOr("REGISTRY_RUN_DEPENDENCIES", "REGISTRY_DEPENDENCIES"))
		if err != nil {
			t.Fatal(err)
		}
		config, err := os.ReadFile(os.Getenv("REGISTRY_CONFIG"))
		if err != nil {
			t.Fatal(err)
		}
		d := Drivers()
		class := d.OSGRun
		if err := os.WriteFile(filepath.Join(out, class+".clas.abap"), []byte(RegistryRunClass(class, inputs, string(config), want, 0, names)), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, class+".clas.testclasses.abap"), []byte(RegistryRunTest(class, traceTrap(names))), 0644); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("REGISTRY_RUN_CORPUS") != "" {
			a4h := d.A4HClass
			var negative *RegistryNegative
			if path := os.Getenv("REGISTRY_RUN_NEGATIVE"); path != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				negative = &RegistryNegative{}
				if err := json.Unmarshal(data, negative); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(out, a4h+".clas.abap"), []byte(RegistryRunCorpusClass(a4h, want, negative, names)), 0644); err != nil {
				t.Fatal(err)
			}
			for _, report := range []string{d.RunReport, d.CleanReport} {
				if err := os.WriteFile(filepath.Join(out, report+".prog.abap"), []byte(RegistryRunReport(report, a4h, false)), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if negative != nil {
				if err := os.WriteFile(filepath.Join(out, d.NegReport+".prog.abap"), []byte(RegistryRunReport(d.NegReport, a4h, true)), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" && os.Getenv("REGISTRY_CLI") != "" {
		if err := os.WriteFile(filepath.Join(out, "zabaplint.prog.abap"), []byte(RegistryCLIReport("zabaplint", names)), 0644); err != nil {
			t.Fatal(err)
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

// traceTrap names the trap exception the cast-trace emission raises.
func traceTrap(names *hir.Names) string {
	if os.Getenv("ABAPITI_CAST_TRACE") == "" {
		return ""
	}
	return names.Get("exception.unexecuted")
}
