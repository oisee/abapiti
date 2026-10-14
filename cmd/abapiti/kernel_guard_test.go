package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/abapfixture"
	"github.com/oisee/abapiti/tsfront"
)

func emitABAP(t *testing.T, p *hir.Program) (map[string]string, error) {
	t.Helper()
	files, err := abap.Emit(p)
	if err == nil {
		abapfixture.Track(t, "cmd/abapiti", files)
	}
	return files, err
}

func checkKernelConstructors(t *testing.T, files map[string]string) {
	t.Helper()
	if issues := abapfixture.KernelViolations(files); len(issues) > 0 {
		t.Fatal(strings.Join(issues, "\n"))
	}
	constructors, supers := 0, 0
	for _, source := range files {
		constructors += strings.Count(strings.ToLower(source), "method constructor.")
		supers += strings.Count(strings.ToLower(source), "super->constructor(")
	}
	t.Logf("scanned %d files, %d constructors, %d superclass calls", len(files), constructors, supers)
}

// Always scan the production pruned default closure in the short gate.
func TestDefaultZabapgitKernelConstructors(t *testing.T) {
	guardRegistryClosure(t, true)
}

func TestAbaplintCoreKernelConstructors(t *testing.T) {
	if os.Getenv("ABAPITI_KERNEL_CORE") == "" {
		t.Skip("set ABAPITI_KERNEL_CORE=1 for the unpruned core scan")
	}
	guardRegistryClosure(t, false)
}

func guardRegistryClosure(t *testing.T, pruned bool) {
	t.Helper()
	t.Setenv("ABAPITI_ASSUME_INT", "1")
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
	var coverage *tsfront.Reachability
	if pruned {
		coverage, err = tsfront.EmbeddedReachability()
		if err != nil {
			t.Fatal(err)
		}
	}
	var l *tsfront.RegistryLowering
	if pruned {
		l, err = tsfront.LowerRegistry(work, append(closure.Files(), tsfront.RegistryHarnessPath), registry, coverage)
	} else {
		// The unpruned core uses the suite's binary64 and fail-closed policy.
		l, err = tsfront.LowerRegistryWithOptions(work, append(closure.Files(), tsfront.RegistryHarnessPath), registry, nil, tsfront.LowerOptions{TrapOnFailure: true})
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Err(); err != nil {
		t.Fatal(err)
	}
	if !pruned {
		measureAndTrapKernelClosure(t, l)
	}
	files, err := emitABAP(t, l.Prog)
	if err != nil {
		t.Fatal(err)
	}
	checkKernelConstructors(t, files)
}

// The full 625-root test closure is caller-materialized with the unchanged
// suite closure tool. This scan includes all test shards and the unpruned core.
func TestAbaplintSuiteKernelConstructors(t *testing.T) {
	dir := os.Getenv("ABAPITI_KERNEL_CLOSURE")
	if dir == "" {
		t.Skip("set ABAPITI_KERNEL_CLOSURE to the full materialized test closure")
	}
	t.Setenv("ABAPITI_INLINE", "classic")
	l, err := tsfront.LowerAbaplintTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Err(); err != nil {
		t.Fatal(err)
	}
	measureAndTrapKernelClosure(t, l)
	files, err := emitABAP(t, l.Prog)
	if err != nil {
		t.Fatal(err)
	}
	checkKernelConstructors(t, files)
}

func measureAndTrapKernelClosure(t *testing.T, l *tsfront.RegistryLowering) {
	t.Helper()
	// Optional evidence uses the production counting tool, before trapping.
	if dir := os.Getenv("ABAPITI_KERNEL_GAP_OUT"); dir != "" {
		gap := tsfront.MeasureABAPTestGap(l)
		data, err := json.MarshalIndent(gap, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, t.Name()+".json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := tsfront.TrapABAPTestRefusals(l); err != nil {
		t.Fatal(err)
	}
}
