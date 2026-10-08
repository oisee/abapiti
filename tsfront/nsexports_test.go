package tsfront

import (
	"path/filepath"
	"testing"
)

// A namespace module's non-class exports are left out of its class-value map
// only when no file reads them through a namespace; a read stays blocking.
func TestNamespaceNonClassExports(t *testing.T) {
	for name, want := range map[string]string{"read": "unsupported-namespace-export", "unread": "note-namespace-export-unread"} {
		t.Run(name, func(t *testing.T) {
			p, err := Load(filepath.Join("testdata", "nsexports", name, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, diags, err := p.Lower([]string{"input.ts", "defs.ts", "alpha.ts", "table.ts"})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diags {
				if d.Category == want {
					return
				}
			}
			t.Fatalf("missing %s: %v", want, diags)
		})
	}
}
