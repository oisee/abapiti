// Command tsdump loads a project from its real tsconfig.json with the
// vendored tsgo compiler, runs the checker, and writes a deterministic JSON
// dump of the selected files (classes, functions, one entry per expression
// with checker types, symbol resolution and inheritance information).
//
// Typical use:
//
//	tsdump -tsconfig ~/dev/abaplint/packages/core/tsconfig.json \
//	       -out lexer-dump.json -check 'src/abap/1_lexer/**/*.ts'
//
// Timings (load / check / dump) are written to stderr.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oisee/abapiti/tsfront"
)

func main() {
	tsconfig := flag.String("tsconfig", "tsconfig.json", "path to the project's tsconfig.json")
	out := flag.String("out", "", "output file for the JSON dump (default: stdout)")
	check := flag.Bool("check", false, "run the checker over the whole program and report diagnostics")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: tsdump [-tsconfig path] [-out file] [-check] pattern...\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	start := time.Now()
	program, err := tsfront.Load(*tsconfig)
	if err != nil {
		fatal(err)
	}
	loadDur := time.Since(start)
	fmt.Fprintf(os.Stderr, "loaded %d source files from %s in %s\n", len(program.SourceFiles()), *tsconfig, loadDur)

	if *check {
		start = time.Now()
		diags := program.CheckerDiagnostics(context.Background())
		fmt.Fprintf(os.Stderr, "checked program in %s: %d diagnostics\n", time.Since(start), len(diags))
		for _, d := range diags {
			fmt.Fprintf(os.Stderr, "  %s: %s TS%d %s\n", d.Loc, d.Category, d.Code, d.Message)
		}
	}

	files, err := expandPatterns(flag.Args())
	if err != nil {
		fatal(err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		fatal(fmt.Errorf("no files matched %v", flag.Args()))
	}

	start = time.Now()
	dumps, err := program.Dump(files)
	if err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "dumped %d files in %s\n", len(dumps), time.Since(start))

	data, err := json.MarshalIndent(dumps, "", "  ")
	if err != nil {
		fatal(err)
	}
	data = append(data, '\n')

	if *out == "" {
		_, err = os.Stdout.Write(data)
	} else {
		err = os.WriteFile(*out, data, 0o666)
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tsdump:", err)
	os.Exit(1)
}

// expandPatterns expands shell-like patterns with ** support (relative to
// the working directory) to sorted, absolute file paths.
func expandPatterns(patterns []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		for _, f := range expandGlob(pattern) {
			abs, err := filepath.Abs(f)
			if err != nil || seen[abs] {
				continue
			}
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out, nil
}

// expandGlob expands a single pattern; "**" matches any number of path
// segments, "*" and "?" one segment (Go's filepath.Match within a segment).
func expandGlob(pattern string) []string {
	segs := strings.Split(filepath.ToSlash(pattern), "/")

	// Longest fixed directory prefix to start the walk at.
	i := 0
	for ; i < len(segs)-1; i++ {
		if strings.ContainsAny(segs[i], "*?[") {
			break
		}
	}
	root := "."
	if i > 0 {
		root = filepath.FromSlash(strings.Join(segs[:i], "/"))
		segs = segs[i:]
	}

	var out []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable directories are skipped, not fatal
		}
		if d.IsDir() {
			// The walk root itself ("." for bare/wildcard patterns, or a
			// directory named by the pattern) is never skipped, even if it
			// is hidden; only directories found below it are.
			if path != root && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		if matchSegments(segs, strings.Split(filepath.ToSlash(rel), "/")) {
			out = append(out, path)
		}
		return nil
	}); err != nil {
		// An unreadable walk root yields no files; the caller reports "no
		// files matched".
		return nil
	}
	return out
}

// matchSegments matches slash-separated path segments against pattern
// segments, where "**" consumes any number of segments.
func matchSegments(pattern, name []string) bool {
	switch {
	case len(pattern) == 0:
		return len(name) == 0
	case pattern[0] == "**":
		for skip := 0; skip <= len(name); skip++ {
			if matchSegments(pattern[1:], name[skip:]) {
				return true
			}
		}
		return false
	case len(name) == 0:
		return false
	default:
		if ok, _ := filepath.Match(pattern[0], name[0]); ok {
			return matchSegments(pattern[1:], name[1:])
		}
		return false
	}
}
