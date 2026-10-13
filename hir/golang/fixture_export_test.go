package golang

import (
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/abapfixture"
)

func emitABAP(t *testing.T, p *hir.Program) (map[string]string, error) {
	t.Helper()
	files, err := abap.Emit(p)
	if err == nil {
		abapfixture.Track(t, "hir/golang", files)
	}
	return files, err
}
