package wasm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/abapsize"
)

func TestGeneratedLineLimit(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/*.wasm")
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Skip("WASM fixtures missing")
	}
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				t.Skipf("fixture missing: %s", path)
			}
			if err != nil {
				t.Fatal(err)
			}
			mod, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []BackendKind{BackendClass, BackendFUGR, BackendHybrid} {
				t.Run(backend.String(), func(t *testing.T) {
					result := CompileWith(mod, "zcl_line_limit", backend, 80)
					checkLineLimit(t, result.Files)
				})
			}
			t.Run("multi-class", func(t *testing.T) {
				result := CompileMultiClass(mod, "zcl_line_limit", 80)
				files := map[string]string{"main.clas.abap": result.MainClass, "runtime.clas.abap": result.RuntimeClass}
				for name, src := range result.ChunkClasses {
					files[name+".clas.abap"] = src
				}
				checkLineLimit(t, files)
			})
		})
	}
}
func checkLineLimit(t *testing.T, files map[string]string) {
	t.Helper()
	r := abapsize.Report(files)
	if len(r.LongLines) > 0 {
		errors := r.Errors()
		if len(errors) > 10 {
			errors = errors[:10]
		}
		t.Fatalf("%d lines exceed 255 characters: %v", len(r.LongLines), errors)
	}
}
