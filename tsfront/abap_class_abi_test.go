package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// These source-to-HIR regressions retain their Node/Go oracles and also inspect
// the actual ABAP boundary. A checked cast must not become a structural copy
// or silently catch a mismatched native reference.
func requireCheckedClassViewABAP(t *testing.T, p *hir.Program) {
	t.Helper()
	files, err := abap.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, src := range files {
		found = found || strings.Contains(src, " ?= ")
	}
	if !found {
		t.Fatal("nominal view lost its ABAP checked-cast boundary")
	}
}
