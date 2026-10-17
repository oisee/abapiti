package tsfront

import (
	"os"
	"path/filepath"
	"strings"
)

// packageSourceNames names project files that live outside the tsconfig
// directory in a separate npm package checkout (reached through tsconfig
// paths, e.g. @abaplint/core mapped to packages/core/src/index.ts) by their
// path inside that package: packages/core/src/config.ts becomes
// src/config.ts, the same name the abaplint build gives it. Generated names,
// locations and source-pinned override keys then do not depend on where the
// checkout lives. A package that contains the tsconfig directory is part of
// the project itself, and an ambiguous name (two packages, or a project file,
// with the same relative path) keeps its tsconfig-relative spelling.
func packageSourceNames(root string, files []string) map[string]string {
	if root == "" {
		return nil
	}
	taken := map[string]int{}
	candidates := map[string]string{}
	pkgRoots := map[string]string{}
	var packageRoot func(dir string) string
	packageRoot = func(dir string) string {
		if r, ok := pkgRoots[dir]; ok {
			return r
		}
		r := ""
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			r = dir
		} else if parent := filepath.Dir(dir); parent != dir {
			r = packageRoot(parent)
		}
		pkgRoots[dir] = r
		return r
	}
	for _, file := range files {
		name := relativeSource(root, file)
		taken[name]++
		if !strings.HasPrefix(name, "../") || !filepath.IsAbs(file) {
			continue
		}
		pkg := packageRoot(filepath.Dir(file))
		if pkg == "" {
			continue
		}
		if rel, err := filepath.Rel(pkg, root); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue // the package contains the project
		}
		rel, err := filepath.Rel(pkg, file)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(filepath.ToSlash(rel), "/node_modules/") {
			continue
		}
		candidates[file] = filepath.ToSlash(rel)
	}
	for _, name := range candidates {
		taken[name]++
	}
	out := map[string]string{}
	for file, name := range candidates {
		if taken[name] == 1 {
			out[file] = name
		}
	}
	return out
}

// sourceName is the stable project name of an absolute source file path.
func (p *Program) sourceName(file string) string {
	if name, ok := p.packageNames[filepath.Clean(file)]; ok {
		return name
	}
	return relativeSource(p.configDir, file)
}
