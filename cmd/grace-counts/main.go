// grace-counts measures singleton-argument specialization opportunities without
// changing HIR. Inputs are exactly the pinned production registry closure.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/tsfront"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err)
	}
	out := flag.String("output", filepath.Join(home, ".cache/grace-counts"), "output directory")
	closure := flag.String("closure", os.Getenv("REGISTRY_CLOSURE"), "optional verified closure directory; otherwise unpack embedded sources")
	flag.Parse()
	if flag.NArg() != 0 {
		fatal(fmt.Errorf("unexpected positional arguments"))
	}
	if err := run(*out, *closure); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func run(out, dir string) error {
	start := time.Now()
	var err error
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if dir != "" {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	if err := os.Setenv("ABAPITI_ASSUME_INT", "1"); err != nil {
		return err
	}
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp(out, "closure-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		if err := unpack(dir); err != nil {
			return err
		}
	}
	manifest := tsfront.EmbeddedRegistryClosure()
	if len(manifest.Sources) != 1538 {
		return fmt.Errorf("closure has %d files, want 1538", len(manifest.Sources))
	}
	if err := manifest.Verify(dir); err != nil {
		return err
	}
	for _, pkg := range tsfront.RegistryNodePackages() {
		if err := pkg.Verify(filepath.Join(dir, "node_modules", pkg.Name)); err != nil {
			return err
		}
	}
	harness := filepath.Join(dir, tsfront.RegistryHarnessPath)
	if err := os.MkdirAll(filepath.Dir(harness), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(harness, tsfront.RegistryHarness(), 0644); err != nil {
		return err
	}
	coverage, err := tsfront.EmbeddedReachability()
	if err != nil {
		return err
	}
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		return err
	}
	lowering, err := tsfront.LowerRegistry(dir, append(manifest.Files(), tsfront.RegistryHarnessPath), registry, coverage)
	if err != nil {
		return err
	}
	if err := lowering.Err(); err != nil {
		return err
	}
	p := lowering.Prog
	if len(p.Classes) != 1927 || len(p.Interfaces) != 73 {
		return fmt.Errorf("changed closure: %d classes, %d interfaces", len(p.Classes), len(p.Interfaces))
	}
	fmt.Fprintf(os.Stderr, "lowered 1538 files + harness: %d classes, %d interfaces; no blocking/verification errors (%s)\n", len(p.Classes), len(p.Interfaces), time.Since(start))
	if os.Getenv("GRACE_COUNTS_MEASURE_STORES") == "1" {
		return measureStores(p, out)
	}
	if os.Getenv("GRACE_COUNTS_MEASURE_INLINE") == "1" {
		return measureInline(p)
	}
	db, err := rewrite.Analyze(p)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Grace: %d methods, %d receiver facts (%s)\n", db.Count("defined"), db.Count("receivers"), time.Since(start))
	sites, methods := count(p, db, literalSource)
	for i := range sites {
		sites[i].Source = strings.TrimPrefix(sites[i].Source, dir+string(filepath.Separator))
	}
	flowSites := flowCounts(p, db, dir)
	flowReport, err := writeFlowOutputs(out, flowSites, db)
	if err != nil {
		return err
	}
	report := tables(sites, methods) + flowReport
	if err := writeOutputs(out, sites, methods, report); err != nil {
		return err
	}
	fmt.Print(report)
	return nil
}

// Source provenance separates literal [e] from [] followed by a later push,
// which has the same collection operations in HIR. Columns are UTF-16 units.
func literalSource(loc string) bool {
	last := strings.LastIndex(loc, ":")
	if last < 0 {
		return false
	}
	prev := strings.LastIndex(loc[:last], ":")
	if prev < 0 {
		return false
	}
	line, e1 := strconv.Atoi(loc[prev+1 : last])
	col, e2 := strconv.Atoi(loc[last+1:])
	if e1 != nil || e2 != nil || line < 1 || col < 1 {
		return false
	}
	b, err := os.ReadFile(loc[:prev])
	if err != nil {
		return false
	}
	lines := strings.Split(string(b), "\n")
	if line > len(lines) {
		return false
	}
	units, offset := 1, 0
	for i, r := range lines[line-1] {
		if units >= col {
			offset = i
			break
		}
		units++
		if r > 0xffff {
			units++
		}
		offset = len(lines[line-1])
	}
	text := strings.TrimSpace(strings.Join(lines[line-1:], "\n")[offset:])
	if !strings.HasPrefix(text, "[") {
		return false
	}
	text = strings.TrimSpace(text[1:])
	for {
		if strings.HasPrefix(text, "/*") {
			end := strings.Index(text[2:], "*/")
			if end < 0 {
				return false
			}
			text = strings.TrimSpace(text[end+4:])
			continue
		}
		if strings.HasPrefix(text, "//") {
			end := strings.Index(text, "\n")
			if end < 0 {
				return false
			}
			text = strings.TrimSpace(text[end+1:])
			continue
		}
		break
	}
	// HIR proves the one element; provenance only rejects empty/spread literals.
	return !strings.HasPrefix(text, "]") && !strings.HasPrefix(text, "...")
}

func unpack(dir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(tsfront.EmbeddedAbaplintArchive()))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || !strings.HasPrefix(h.Name, "packages/core/src/") {
			continue
		}
		rel := strings.TrimPrefix(h.Name, "packages/core/")
		if filepath.Clean(rel) != rel || strings.Contains(rel, "..") {
			return fmt.Errorf("unsafe archive path: %s", h.Name)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, b, 0644); err != nil {
			return err
		}
	}
	for _, pkg := range tsfront.RegistryNodePackages() {
		if err := pkg.WriteEmbedded(filepath.Join(dir, "node_modules", pkg.Name)); err != nil {
			return err
		}
	}
	config := `{"compilerOptions":{"module":"commonjs","target":"es2020","lib":["es2020"],"noEmit":true,"skipLibCheck":true,"strictNullChecks":true,"strictFunctionTypes":true,"noImplicitAny":true,"strictPropertyInitialization":false},"include":["src/**/*.ts","harness/**/*.ts"]}`
	return os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(config), 0644)
}

// walk matches Grace's structural site identities, including expression Seq.
func walk(s *hir.Stmt, path string, loop int, stmt func(*hir.Stmt, string, int), expr func(*hir.Expr, string, int)) {
	if s == nil {
		return
	}
	stmt(s, path, loop)
	if s.Kind == hir.Block {
		for i, b := range s.List {
			walk(b, fmt.Sprintf("%s/s%d", path, i), loop, stmt, expr)
		}
		return
	}
	walkExpr(s.X, path+"/x", loop, stmt, expr)
	walkExpr(s.Y, path+"/y", loop, stmt, expr)
	depth := loop
	if s.Kind == hir.ForEach || s.Kind == hir.While {
		depth++
	}
	walk(s.Body, path+"/body", depth, stmt, expr)
	walk(s.Else, path+"/else", loop, stmt, expr)
}
func walkExpr(e *hir.Expr, path string, loop int, stmt func(*hir.Stmt, string, int), expr func(*hir.Expr, string, int)) {
	if e == nil {
		return
	}
	if e.Kind == hir.Seq && e.Stmt != nil {
		for i, s := range e.Stmt.List {
			walk(s, fmt.Sprintf("%s/seq/s%d", path, i), loop, stmt, expr)
		}
	}
	walkExpr(e.X, path+"/0", loop, stmt, expr)
	walkExpr(e.Y, path+"/1", loop, stmt, expr)
	walkExpr(e.Z, path+"/2", loop, stmt, expr)
	for i, a := range e.Args {
		walkExpr(a, fmt.Sprintf("%s/arg%d", path, i), loop, stmt, expr)
	}
	expr(e, path, loop)
}
