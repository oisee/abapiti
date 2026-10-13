package abap

import (
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/abapfixture"
)

func fixtureOutputDir(t *testing.T) string {
	t.Helper()
	return abapfixture.OutputDir(t, "hir/abap")
}

func emitABAP(t *testing.T, p *hir.Program) (map[string]string, error) {
	t.Helper()
	files, err := Emit(p)
	if err == nil {
		abapfixture.Track(t, "hir/abap", files)
	}
	return files, err
}

func emitABAPNamed(t *testing.T, p *hir.Program) (map[string]string, *hir.Names, error) {
	t.Helper()
	files, names, err := EmitNamed(p)
	if err == nil {
		abapfixture.Track(t, "hir/abap", files)
	}
	return files, names, err
}
