package llvm

import (
	"testing"

	"github.com/oisee/abapiti/internal/abapfixture"
)

func compileABAP(t *testing.T, mod *Module, name string) string {
	t.Helper()
	src := Compile(mod, name)
	abapfixture.Track(t, "llvm", map[string]string{name + ".prog.abap": src})
	return src
}
