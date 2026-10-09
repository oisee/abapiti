package tsfront

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
)

// The full closure uses the production CLI lowering inputs. Skip it with
// -short: independent reference joins are expensive on thousands of methods.
func TestGraceFullRegistryClosure(t *testing.T) {
	if testing.Short() {
		t.Skip("full pinned 1538-file closure requires non-short tests")
	}
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	start := time.Now()
	defer func() { t.Logf("full Grace closure runtime: %s", time.Since(start)) }()
	dir := os.Getenv("REGISTRY_CLOSURE")
	if dir == "" {
		if len(EmbeddedAbaplintArchive()) == 0 {
			t.Skip("pinned full closure inputs absent")
		}
		dir = t.TempDir()
		gz, e := gzip.NewReader(bytes.NewReader(EmbeddedAbaplintArchive()))
		if e != nil {
			t.Fatal(e)
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			h, e := tr.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			if h.Typeflag != tar.TypeReg || !strings.HasPrefix(h.Name, "packages/core/src/") {
				continue
			}
			rel := strings.TrimPrefix(h.Name, "packages/core/")
			if filepath.Clean(rel) != rel || strings.Contains(rel, "..") {
				t.Fatalf("unsafe pinned archive path: %s", h.Name)
			}
			target := filepath.Join(dir, rel)
			if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
				t.Fatal(e)
			}
			b, e := io.ReadAll(tr)
			if e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(target, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
		for _, pkg := range RegistryNodePackages() {
			if e := pkg.WriteEmbedded(filepath.Join(dir, "node_modules", pkg.Name)); e != nil {
				t.Fatal(e)
			}
		}
		config := `{"compilerOptions":{"module":"commonjs","target":"es2020","lib":["es2020"],"noEmit":true,"skipLibCheck":true,"strictNullChecks":true,"strictFunctionTypes":true,"noImplicitAny":true,"strictPropertyInitialization":false},"include":["src/**/*.ts","harness/**/*.ts"]}`
		if e := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(config), 0644); e != nil {
			t.Fatal(e)
		}
	}
	manifest := EmbeddedRegistryClosure()
	if len(manifest.Sources) != 1538 {
		t.Fatalf("full closure has %d files, want 1538", len(manifest.Sources))
	}
	if e := manifest.Verify(dir); e != nil {
		if os.IsNotExist(e) {
			t.Skipf("full closure input absent: %v", e)
		}
		t.Fatal(e)
	}
	harness := filepath.Join(dir, RegistryHarnessPath)
	if e := os.MkdirAll(filepath.Dir(harness), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(harness, RegistryHarness(), 0644); e != nil {
		t.Fatal(e)
	}
	coverage, e := EmbeddedReachability()
	if e != nil {
		t.Fatal(e)
	}

	registry, e := RegistryOverrides()
	if e != nil {
		t.Fatal(e)
	}
	lowering, e := LowerRegistry(dir, append(manifest.Files(), RegistryHarnessPath), registry, coverage)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("lowered %d files in %s: %d classes, %d blocking diagnostics, %d verification errors", len(manifest.Sources), time.Since(start), len(lowering.Prog.Classes), len(lowering.Blocking), len(lowering.Verification))
	if e := lowering.Err(); e != nil {
		t.Fatal(e)
	}
	if len(lowering.Prog.Classes) != 1927 || len(lowering.Prog.Interfaces) != 73 {
		t.Fatalf("changed closure: %d classes, %d interfaces", len(lowering.Prog.Classes), len(lowering.Prog.Interfaces))
	}
	db, e := rewrite.Analyze(lowering.Prog)
	if e != nil {
		t.Fatal(e)
	}
	report := rewrite.Report(db)
	t.Log("full closure facts\n" + report[strings.Index(report, "summary\n"):])
	if out := os.Getenv("GRACE_FULL_FACTS_OUT"); out != "" {
		if e := os.WriteFile(out, []byte(report), 0644); e != nil {
			t.Fatal(e)
		}
	}
	gracecheck.CheckFull(t, lowering.Prog)
}
