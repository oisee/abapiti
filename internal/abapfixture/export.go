// Package abapfixture preserves emitted ABAP for the compile-only CI gate.
package abapfixture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Track exports at test cleanup, after callers have attached ABAP Unit classes.
// Each emission is independent: generated runtime names overlap between programs.
// Failed emissions (nil maps) produce no fixture. Export errors fail the test.
func Track(t *testing.T, pkg string, files map[string]string) {
	t.Helper()
	base := os.Getenv("ABAPITI_ABAP_FIXTURE_DIR")
	if base == "" || files == nil {
		return
	}
	key := filepath.Join(pkg, t.Name())
	mu.Lock()
	counterKey := filepath.Join(base, key)
	counts[counterKey]++
	n := counts[counterKey]
	mu.Unlock()
	dir := filepath.Join(base, key, fmt.Sprintf("emit-%03d", n))
	t.Cleanup(func() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Errorf("ABAP fixture %s: %v", key, err)
			return
		}
		for name, src := range files {
			if filepath.Base(name) != name {
				t.Errorf("ABAP fixture %s: invalid filename %q", key, name)
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
				t.Errorf("ABAP fixture %s: %v", key, err)
			}
		}
	})
}

// OutputDir preserves the legacy export contract, while enabling conditional
// unit/driver generation for the new gate. A temporary directory per call keeps
// old exporters with incompatible layouts from overwriting each other's files.
func OutputDir(t *testing.T, pkg string) string {
	t.Helper()
	legacy := os.Getenv("ABAPITI_TEST_OUT")
	if os.Getenv("ABAPITI_ABAP_FIXTURE_DIR") == "" {
		return legacy
	}
	mu.Lock()
	if dir, ok := outputDirs[t]; ok {
		mu.Unlock()
		return dir
	}
	dir := legacy
	if dir == "" {
		dir = t.TempDir()
	}
	outputDirs[t] = dir
	mu.Unlock()
	CollectDir(t, pkg, dir)
	return dir
}

// CollectDir exports ABAP from a CLI or legacy generator's output directory
// at cleanup, after its final unit classes and companion files are written.
func CollectDir(t *testing.T, pkg, dir string) {
	t.Helper()
	if os.Getenv("ABAPITI_ABAP_FIXTURE_DIR") == "" {
		return
	}
	t.Cleanup(func() {
		var collect func(string) error
		collect = func(path string) error {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			files := make(map[string]string)
			for _, entry := range entries {
				name := filepath.Join(path, entry.Name())
				if entry.IsDir() {
					if err := collect(name); err != nil {
						return err
					}
				} else if strings.HasSuffix(entry.Name(), ".abap") || strings.HasSuffix(entry.Name(), ".xml") {
					src, err := os.ReadFile(name)
					if err != nil {
						return err
					}
					files[entry.Name()] = string(src)
				}
			}
			if len(files) > 0 {
				Track(t, pkg, files)
			}
			return nil
		}
		if err := collect(dir); err != nil {
			t.Errorf("ABAP fixture %s: %v", t.Name(), err)
		}
	})
}

var mu sync.Mutex
var counts = make(map[string]int)
var outputDirs = make(map[*testing.T]string)
