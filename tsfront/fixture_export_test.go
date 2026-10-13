package tsfront

import (
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/abapfixture"
)

func fixtureOutputDir(t *testing.T) string {
	t.Helper()
	return abapfixture.OutputDir(t, "tsfront")
}

func emitABAPTestsOSG(t *testing.T, dir string, lowering *RegistryLowering) (map[string]string, error) {
	t.Helper()
	files, err := EmitAbaplintTestsOSG(dir, lowering)
	if err == nil {
		abapfixture.Track(t, "tsfront", files)
	}
	return files, err
}

func emitABAP(t *testing.T, p *hir.Program) (map[string]string, error) {
	t.Helper()
	files, err := abap.Emit(p)
	if err == nil {
		abapfixture.Track(t, "tsfront", files)
	}
	return files, err
}

func emitABAPNamed(t *testing.T, p *hir.Program) (map[string]string, *hir.Names, error) {
	t.Helper()
	files, names, err := abap.EmitNamed(p)
	if err == nil {
		abapfixture.Track(t, "tsfront", files)
	}
	return files, names, err
}
