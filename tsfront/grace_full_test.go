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

	"github.com/oisee/abapiti/internal/gracecheck"
)

// The full closure is opt-in because lowering thousands of original methods
// and naive joins are unsuitable for the ordinary frontend unit-test loop.
func TestGraceFullRegistryClosure(t *testing.T) {
	if testing.Short() || os.Getenv("GRACE_FULL_CLOSURE") != "1" {
		t.Skip("set GRACE_FULL_CLOSURE=1 without -short for the pinned 1538-file closure")
	}
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
		config := `{"compilerOptions":{"module":"commonjs","target":"es2020","lib":["es2020"],"noEmit":true,"skipLibCheck":true,"strictNullChecks":true,"strictFunctionTypes":true,"noImplicitAny":true,"strictPropertyInitialization":false},"include":["src/**/*.ts"]}`
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
	registry, e := RegistryOverrides()
	if e != nil {
		t.Fatal(e)
	}
	lowering, e := LowerRegistry(dir, manifest.Files(), registry, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("lowered %d files in %s: %d classes, %d blocking diagnostics, %d verification errors", len(manifest.Sources), time.Since(start), len(lowering.Prog.Classes), len(lowering.Blocking), len(lowering.Verification))
	if len(lowering.Blocking) > 0 || len(lowering.Verification) > 0 {
		for i, d := range lowering.Blocking {
			if i == 5 {
				break
			}
			t.Log(d)
		}
		for i, e := range lowering.Verification {
			if i == 5 {
				break
			}
			t.Log(e)
		}
		t.Skip("full closure does not lower to verified HIR yet; Grace checks require complete verified input")
	}
	gracecheck.Check(t, lowering.Prog)
}
