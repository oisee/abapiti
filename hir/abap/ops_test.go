package abap

import (
	"testing"

	"github.com/oisee/abapiti/hir/golang"
)

// Every catalogue operation must be explicitly reviewed by both backends,
// including repeat/match and the small builtins added on main.
func TestRuntimeOpsSupportedByEveryBackend(t *testing.T) {
	for name, catalogue := range map[string]func() ([]string, []string){
		"abap":   RuntimeOps,
		"golang": golang.RuntimeOps,
	} {
		supported, unsupported := catalogue()
		if len(supported) == 0 || len(unsupported) != 0 {
			t.Errorf("%s: supported=%d unsupported=%v", name, len(supported), unsupported)
		}
	}
}
