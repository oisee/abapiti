package tsfront

import (
	"fmt"
	"path/filepath"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// Validate all selected source spans before lowering. A deleted or ambiguous
// target is stale too; no changed source can silently fall back to generic code.
func (l *lowerer) validateOverrides(files []string, registry *overrides.Registry) error {
	if len(registry.Inventory()) == 0 {
		return nil
	}
	seen := map[string]int{}
	selected := map[string]bool{}
	for _, name := range files {
		f, ok := l.prog.File(name)
		if !ok {
			continue
		}
		rel, _ := filepath.Rel(l.prog.configDir, f.FileName())
		rel = filepath.ToSlash(rel)
		selected[rel] = true
		var walk func(*ast.Node, string)
		var failure error
		walk = func(n *ast.Node, symbol string) {
			if n == nil || failure != nil {
				return
			}
			switch n.Kind {
			case ast.KindClassDeclaration, ast.KindInterfaceDeclaration, ast.KindFunctionDeclaration, ast.KindMethodDeclaration:
				if n.Name() != nil && (n.Name().Kind == ast.KindIdentifier || n.Name().Kind == ast.KindStringLiteral) {
					if n.Kind == ast.KindMethodDeclaration && symbol != "" {
						symbol += "." + n.Name().Text()
					} else {
						symbol = n.Name().Text()
					}
				}
			}
			key := overrides.Key{File: rel, Symbol: symbol, Kind: n.Kind.String()}
			start := scanner.GetTokenPosOfNode(n, f, false)
			entry, ok, err := registry.Lookup(key, f.Text()[start:n.End()], locString(f, start))
			if err != nil {
				failure = err
				return
			}
			if ok {
				seen[entry.ID]++
				l.overrides[n] = entry
			}
			n.ForEachChild(func(c *ast.Node) bool { walk(c, symbol); return false })
		}
		for _, n := range f.Statements.Nodes {
			walk(n, "")
		}
		if failure != nil {
			return failure
		}
	}
	for _, entry := range registry.Inventory() {
		if selected[entry.Key.File] && seen[entry.ID] != 1 {
			return fmt.Errorf("override %s is stale: target occurs %d times at %s (%s)", entry.ID, seen[entry.ID], entry.Key.File, entry.Key.Symbol)
		}
	}
	return nil
}
