package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
)

// Exercise the CLI's full, unpruned core and fail-closed policy. The pruned
// Grace closure cannot detect regressions in interfaces used only outside its
// workload. This takes about a minute, so it shares the full-closure opt-in.
func TestAbaplintCoreTrappedBodies(t *testing.T) {
	if testing.Short() || os.Getenv("ABAPITI_GRACE_FULL") != "1" {
		t.Skip("full core trap baseline requires ABAPITI_GRACE_FULL=1 without -short")
	}
	// The CLI's default core build retains binary64 numbers.
	t.Setenv("ABAPITI_ASSUME_INT", "")
	src, err := embeddedAbaplint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	closure := tsfront.EmbeddedRegistryClosure()
	if err := materializeClosure(src, closure, work); err != nil {
		t.Fatal(err)
	}
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	lowering, err := tsfront.LowerRegistryWithOptions(work, append(closure.Files(), tsfront.RegistryHarnessPath), registry, nil, tsfront.LowerOptions{
		TrapOnFailure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lowering.Err(); err != nil {
		t.Fatal(err)
	}
	// Pin every trapped body, including intentional debug refusals. A Go
	// emission contains each failure twice (ordinary and profile variants).
	// Selfhost also has a CodeActions missing-expression failure, separate
	// from the two emitted LanguageServer assignment-mismatch occurrences.
	want := map[string]string{
		"src/abap/5_syntax/spaghetti_scope.ts.SpaghettiScopeNode.debugDescription": "src/abap/5_syntax/spaghetti_scope.ts:37",
		"src/abap/types/_typed_identifier.ts.TypedIdentifier.debugDescription":     "src/abap/types/_typed_identifier.ts:35",
		"src/objects/_abap_object.ts.ABAPObject.debugDescription":                  "src/objects/_abap_object.ts:20",
		"src/lsp/code_actions.ts.CodeActions.find":                                 "lowering trap: src/lsp/code_actions.ts:17:3: HIR verification: node # (src/lsp/code_actions.ts:17:3): missing expression",
		"src/lsp/language_server.ts.LanguageServer.prepareRename":                  "lowering trap: src/lsp/language_server.ts:83:3: HIR verification: node # (src/lsp/language_server.ts:84:5): assignment type mismatch",
	}
	ids := regexp.MustCompile(`node [0-9]+`)
	for _, c := range lowering.Prog.Classes {
		for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if m == nil || m.Body == nil || m.Body.Kind != hir.Block || len(m.Body.List) != 1 || m.Body.List[0].Kind != hir.Trap {
				continue
			}
			key := c.Name + "." + m.Name
			got := ids.ReplaceAllString(m.Body.List[0].Name, "node #")
			if expected, ok := want[key]; !ok || got != expected {
				t.Errorf("unexpected trapped body %s: %s", key, got)
			}
			delete(want, key)
		}
	}
	for key := range want {
		t.Errorf("pinned trap disappeared: %s; review and update the allow-list", key)
	}
}
