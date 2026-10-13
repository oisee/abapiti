//go:build wasmlegacy

package wasm

import (
	"os"
	"testing"

	"github.com/oisee/abapiti/internal/abapfixture"
)

func compileABAPWith(t *testing.T, mod *Module, name string, backend BackendKind, size int) (*BackendResult, error) {
	t.Helper()
	out, err := CompileWith(mod, name, backend, size)
	if err == nil {
		abapfixture.Track(t, "wasm", out.Files)
	}
	return out, err
}

func compileABAPMultiClass(t *testing.T, mod *Module, name string, size int) (*CompileResult, error) {
	t.Helper()
	out, err := CompileMultiClass(mod, name, size)
	if err == nil && os.Getenv("ABAPITI_ABAP_FIXTURE_DIR") != "" {
		files, exportErr := out.Files(name)
		if exportErr != nil {
			t.Errorf("ABAP fixture %s: %v", t.Name(), exportErr)
		} else {
			abapfixture.Track(t, "wasm", files)
		}
	}
	return out, err
}
