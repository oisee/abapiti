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
		if len(supported) == 0 {
			t.Errorf("%s: empty supported catalogue", name)
		}
		for _, op := range unsupported {
			if name != "abap" || !unsupportedOps[op] {
				t.Errorf("%s: unreviewed unsupported op %s", name, op)
			}
		}
		if name == "abap" && len(unsupported) != len(unsupportedOps) {
			t.Errorf("ABAP unsupported catalogue drift: %v", unsupported)
		}
	}
}
