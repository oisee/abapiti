package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The glob patterns are relative to the working directory; the fixture
// projects live in ../../testdata.
const fixtureProject = "../../testdata/project"
const fixtureConfig = "../../testdata/config"

func basenames(t *testing.T, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	return out
}

func hasAll(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, b := range got {
		set[b] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("%s missing from %v", w, got)
		}
	}
}

// Regression test: the walk root "." (used for bare and wildcard patterns)
// used to be skipped as a "hidden" directory, so '*.ts', '**/*.ts' and bare
// file names matched nothing when run from the project directory.
func TestExpandGlobFromProjectDirectory(t *testing.T) {
	t.Chdir(fixtureProject)

	got, err := expandPatterns([]string{"*.ts"})
	if err != nil {
		t.Fatalf("expandPatterns *.ts: %v", err)
	}
	hasAll(t, basenames(t, got), "base.ts", "derived.ts", "box.ts")

	got, err = expandPatterns([]string{"**/*.ts"})
	if err != nil {
		t.Fatalf("expandPatterns **/*.ts: %v", err)
	}
	hasAll(t, basenames(t, got), "base.ts", "derived.ts", "box.ts")

	got, err = expandPatterns([]string{"base.ts"})
	if err != nil {
		t.Fatalf("expandPatterns base.ts: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "base.ts" {
		t.Errorf("bare base.ts: got %v", got)
	}
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Errorf("result %q is not absolute", p)
		}
	}
}

// Patterns may also name a fixed directory prefix; hidden directories below
// the root are still skipped, the root itself is not.
func TestExpandGlobWithDirectoryPrefix(t *testing.T) {
	t.Chdir(fixtureConfig)

	got, err := expandPatterns([]string{"keepdir/*.ts"})
	if err != nil {
		t.Fatalf("expandPatterns keepdir/*.ts: %v", err)
	}
	hasAll(t, basenames(t, got), "keep2.ts")

	got, err = expandPatterns([]string{"**/*.ts"})
	if err != nil {
		t.Fatalf("expandPatterns **/*.ts: %v", err)
	}
	hasAll(t, basenames(t, got), "keep.ts", "keep2.ts", "gone.ts", "skip.ts")

	got, err = expandPatterns([]string{"*.tsx"})
	if err != nil {
		t.Fatalf("expandPatterns *.tsx: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("*.tsx: got %v, want no matches", got)
	}
}

// Duplicated patterns must not produce duplicated files.
func TestExpandGlobDeduplicates(t *testing.T) {
	t.Chdir(fixtureProject)
	got, err := expandPatterns([]string{"*.ts", "**/*.ts", "base.ts"})
	if err != nil {
		t.Fatalf("expandPatterns: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range got {
		if seen[p] {
			t.Errorf("duplicate %s", p)
		}
		seen[p] = true
	}
	hasAll(t, basenames(t, got), "base.ts", "derived.ts", "box.ts")
}

// matchSegments: "**" spans directory levels, "*" stays within one segment.
func TestMatchSegments(t *testing.T) {
	for _, tc := range []struct {
		pattern, name string
		want          bool
	}{
		{"*.ts", "a.ts", true},
		{"*.ts", "a.tsx", false},
		{"*.ts", "dir/a.ts", false},
		{"**/*.ts", "a.ts", true},
		{"**/*.ts", "dir/a.ts", true},
		{"**/*.ts", "dir/sub/a.ts", true},
		{"src/**/*.ts", "src/a.ts", true},
		{"src/**/*.ts", "src/sub/a.ts", true},
		{"src/**/*.ts", "other/a.ts", false},
		{"a?c.ts", "abc.ts", true},
		{"a?c.ts", "ac.ts", false},
	} {
		if got := matchSegments(strings.Split(tc.pattern, "/"), strings.Split(tc.name, "/")); got != tc.want {
			t.Errorf("matchSegments(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}
