package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// lowerStmtsClosure lowers the vendored statement-parser closure and reports
// the diagnostics by category. Stage gate: zero blocking diagnostics.
func lowerStmtsClosure(t *testing.T) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	p, err := Load(filepath.Join("testdata", "stmts", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	files := stmtsLowerFiles(t)
	start := time.Now()
	prog, diags, err := p.Lower(files)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	t.Logf("lowered %d files in %s: %d classes, %d interfaces, %d diagnostics",
		len(files), time.Since(start), len(prog.Classes), len(prog.Interfaces), len(diags))
	return prog, diags
}

func TestLowerStatementsClosureExploratory(t *testing.T) {
	if os.Getenv("STMTS_EXPLORE") == "" {
		t.Skip("set STMTS_EXPLORE=1 to run the exploratory lowering")
	}
	prog, diags := lowerStmtsClosure(t)
	inventory := overrides.Abaplint().Inventory()
	t.Logf("override inventory: count=%d", len(inventory))
	for _, e := range inventory {
		t.Logf("override %s: %s %s %s — %s", e.ID, e.Key.File, e.Key.Symbol, e.Key.Kind, e.Rationale)
	}
	byCategory := map[string]int{}
	examples := map[string]string{}
	for _, d := range diags {
		byCategory[d.Category]++
		if _, ok := examples[d.Category]; !ok {
			examples[d.Category] = d.String()
		}
	}
	keys := make([]string, 0, len(byCategory))
	for k := range byCategory {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%4d %s  e.g. %s", byCategory[k], k, examples[k])
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		os.MkdirAll(out, 0o755)
		os.WriteFile(filepath.Join(out, "stmts.hir.txt"), []byte(hir.Dump(prog)), 0o644)
		var list []string
		for _, d := range diags {
			list = append(list, d.String())
		}
		os.WriteFile(filepath.Join(out, "diags.txt"), []byte(strings.Join(list, "\n")), 0o644)
	}
	errors := hir.Verify(prog)
	t.Logf("HIR verification: %d errors", len(errors))
	byRule := map[string]int{}
	for _, err := range errors {
		rule := err.(hir.Error).Message
		for _, prefix := range []string{"expression type mismatch", "unresolved local", "unresolved method", "unresolved field", "invalid override", "missing interface implementation", "invalid parameter", "invalid local", "unsupported runtime op"} {
			if strings.HasPrefix(rule, prefix) {
				rule = prefix
				break
			}
		}
		byRule[rule]++
	}
	var rules []string
	for rule := range byRule {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	var groups []string
	for _, rule := range rules {
		line := fmt.Sprintf("%4d %s", byRule[rule], rule)
		groups = append(groups, line)
		t.Logf("verify rule: %s", line)
	}
	if os.Getenv("STMTS_GATE") != "" {
		for _, d := range diags {
			if !strings.HasPrefix(d.Category, "note-") {
				t.Errorf("blocking: %s", d)
			}
		}
		for _, err := range errors {
			t.Errorf("verify: %v", err)
		}
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		os.WriteFile(filepath.Join(out, "verify-groups.txt"), []byte(strings.Join(groups, "\n")), 0o644)
		var lines []string
		for _, err := range errors {
			lines = append(lines, err.Error())
		}
		os.WriteFile(filepath.Join(out, "verify.txt"), []byte(strings.Join(lines, "\n")), 0o644)
	}
}
