package tsfront

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// stmtsDir is the vendored statement-parser closure.
func stmtsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("testdata", "stmts")
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err != nil {
		t.Skipf("vendored stmts closure missing: %v", err)
	}
	return dir
}

// stmtsLowerFiles lists the files to lower: the vendored closure minus the
// files that exist only for type checking (objects/, abap_file, 3_structures,
// 4_file_information, 5_syntax, types/, abap_parser, xml_utils, ...). The
// harness is lowered last, after what it drives.
func stmtsLowerFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(stmtsDir(t), "src")
	lowered := map[string]bool{
		"position.ts": true, "virtual_position.ts": true, "version.ts": true,
		"files/_ifile.ts": true, "files/memory_file.ts": true, "files/_abstract_file.ts": true,
		"_iregistry.ts": true, "_imacro_references.ts": true,
		"abap/artifacts.ts":                          true,
		"abap/3_structures/structures/_structure.ts": true, "abap/3_structures/structures/_structure_runnable.ts": true, "abap/3_structures/structures/_match.ts": true,
	}
	for _, d := range []string{"abap/1_lexer", "abap/2_statements", "abap/nodes"} {
		filepath.WalkDir(filepath.Join(root, filepath.FromSlash(d)), func(path string, e os.DirEntry, err error) error {
			if err == nil && !e.IsDir() && strings.HasSuffix(path, ".ts") {
				rel, _ := filepath.Rel(root, path)
				lowered[filepath.ToSlash(rel)] = true
			}
			return nil
		})
	}
	if os.Getenv("STRUCTURES_EMIT") != "" {
		filepath.WalkDir(filepath.Join(root, "abap", "3_structures"), func(path string, e os.DirEntry, err error) error {
			if err == nil && !e.IsDir() && strings.HasSuffix(path, ".ts") {
				rel, _ := filepath.Rel(root, path)
				lowered[filepath.ToSlash(rel)] = true
			}
			return nil
		})
		lowered["issue.ts"] = true
		lowered["severity.ts"] = true
		lowered["abap/4_file_information/_identifier.ts"] = true
	}
	var out []string
	for rel := range lowered {
		if strings.HasSuffix(rel, "index.ts") && strings.Contains(rel, "3_structures") {
			continue
		}
		out = append(out, "src/"+rel)
	}
	// deterministic order: sorted; the harness goes last.
	sort.Strings(out)
	out = append(out, "harness/test_file.ts", "harness/statements_dump.ts")
	if os.Getenv("STRUCTURES_EMIT") != "" {
		out = append(out, "harness/structures_dump.ts")
	}
	return out
}

func TestLoadStatementsClosure(t *testing.T) {
	start := time.Now()
	p, err := Load(filepath.Join(stmtsDir(t), "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Logf("load: %s", time.Since(start))
	files := stmtsLowerFiles(t)
	t.Logf("%d files to lower", len(files))
	for _, f := range files {
		if _, ok := p.File(f); !ok {
			t.Errorf("file %s not part of the program", f)
		}
	}
	diags := p.CheckerDiagnostics(context.Background())
	allowed := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "fast-xml-parser") {
			allowed++
			continue
		}
		t.Logf("checker: %s %s: %s", d.Category, d.Loc, d.Message)
	}
	t.Logf("%d checker diagnostics (%d allowed external-module)", len(diags), allowed)
}
