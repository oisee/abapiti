package tsfront

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// statementNodes is an opt-in view; the checker and pinned source AST stay
// intact, including declarations which only participate in type checking.
func (l *lowerer) statementNodes(f *ast.SourceFile) []*ast.Node {
	if l.retained == nil {
		return f.Statements.Nodes
	}
	var nodes []*ast.Node
	for _, n := range f.Statements.Nodes {
		if l.retained[n] || n.Kind == ast.KindImportDeclaration || n.Kind == ast.KindExportDeclaration {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// pruneDeclarations follows checker-resolved references rather than import
// filenames. Excluded bodies contribute signatures only. Module initializers
// and unmapped callable bodies remain conservative roots. Namespace values
// retain every export, including re-exports and class constructors.
func (l *lowerer) pruneDeclarations(files []string) ([]string, error) {
	declarations := map[string]*ast.Node{}
	owners := map[*ast.Node]*ast.SourceFile{}
	top := map[*ast.Node]*ast.Node{}
	key := func(n *ast.Node) string {
		f := ast.GetSourceFileOfNode(n)
		return fmt.Sprintf("%s:%d", f.FileName(), scanner.GetTokenPosOfNode(n, f, false))
	}
	for _, name := range files {
		f, _ := l.prog.File(name)
		for _, n := range f.Statements.Nodes {
			declarations[key(n)] = n
			owners[n] = f
			var assign func(*ast.Node)
			assign = func(c *ast.Node) {
				top[c] = n
				c.ForEachChild(func(child *ast.Node) bool { assign(child); return false })
			}
			assign(n)
		}
	}
	retained := map[*ast.Node]bool{}
	values := map[*ast.Node]bool{}
	var queue []*ast.Node
	keep := func(n *ast.Node, value bool) {
		if n == nil {
			return
		}
		root := top[n]
		if root == nil {
			// Checkers may expose distinct AST views; canonicalize by source position.
			for n.Parent != nil && n.Parent.Kind != ast.KindSourceFile {
				n = n.Parent
			}
			root = declarations[key(n)]
		}
		if root != nil && (!retained[root] || value && !values[root]) {
			values[root] = values[root] || value
			retained[root] = true
			queue = append(queue, root)
		}
	}
	for n := range owners {
		switch n.Kind {
		case ast.KindClassDeclaration, ast.KindFunctionDeclaration:
			var root func(*ast.Node)
			root = func(c *ast.Node) {
				if c.Body() != nil && (c.Kind == ast.KindMethodDeclaration || c.Kind == ast.KindConstructor || c.Kind == ast.KindGetAccessor || c.Kind == ast.KindSetAccessor || c.Kind == ast.KindFunctionDeclaration) {
					if _, trapped := l.unexecuted[c]; !trapped {
						keep(n, true)
					}
					return
				}
				c.ForEachChild(func(child *ast.Node) bool { root(child); return false })
			}
			root(n)
		case ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindEnumDeclaration, ast.KindImportDeclaration, ast.KindExportDeclaration, ast.KindEmptyStatement:
			// These declarations have no independent execution root.
		default:
			// Includes module variable initializers and any unsupported side effect.
			keep(n, true)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		f := owners[n]
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		seenTypes := map[*checker.Type]bool{}
		var typeEdges func(*checker.Type)
		typeEdges = func(t *checker.Type) {
			if t == nil || seenTypes[t] {
				return
			}
			seenTypes[t] = true
			if sym := t.Symbol(); sym != nil {
				for _, decl := range sym.Declarations {
					keep(decl, false)
				}
			}
			if alias := t.Alias(); alias != nil && alias.Symbol() != nil {
				for _, decl := range alias.Symbol().Declarations {
					keep(decl, false)
				}
			}
			if t.Flags()&(checker.TypeFlagsUnionOrIntersection|checker.TypeFlagsTemplateLiteral) != 0 {
				for _, part := range t.Types() {
					typeEdges(part)
				}
			}
			if t.Flags()&checker.TypeFlagsObject != 0 && t.ObjectFlags()&checker.ObjectFlagsReference != 0 {
				for _, arg := range ck.GetTypeArguments(t) {
					typeEdges(arg)
				}
			}
		}
		var walk func(*ast.Node)
		walk = func(c *ast.Node) {
			if c == nil {
				return
			}
			if e, ok := l.overrides[c]; ok && e.Method != nil {
				for _, ref := range e.References {
					for decl, owner := range owners {
						rel, _ := filepath.Rel(l.prog.configDir, owner.FileName())
						if filepath.ToSlash(rel) == ref.File && decl.Name() != nil && decl.Name().Text() == ref.Symbol && decl.Kind.String() == ref.Kind {
							keep(decl, ref.Kind == "KindClassDeclaration")
						}
					}
				}
				// Replacement signatures/values have explicit dependencies. Original
				// reflective bodies must not keep their namespace inventories alive.
				return
			}
			valueReference := values[n]
			for parent := c.Parent; parent != nil && parent != n; parent = parent.Parent {
				if ast.IsTypeNode(parent) || parent.Kind == ast.KindInterfaceDeclaration || parent.Kind == ast.KindTypeAliasDeclaration {
					valueReference = false
					break
				}
				if parent.Kind == ast.KindHeritageClause && parent.AsHeritageClause().Token == ast.KindImplementsKeyword {
					valueReference = false
					break
				}
			}
			if c.Kind == ast.KindIdentifier && (c.Parent == nil || c.Parent.Name() != c) {
				sym := l.resolve(c)
				if sym != nil {
					for _, decl := range sym.Declarations {
						keep(decl, valueReference)
					}
					if sym.Flags&ast.SymbolFlagsModule != 0 {
						for _, export := range ck.GetExportsOfModule(sym) {
							if export.Flags&ast.SymbolFlagsAlias != 0 {
								if target, ok := ck.ResolveAlias(export); ok {
									export = target
								}
							}
							for _, decl := range export.Declarations {
								keep(decl, valueReference)
							}
						}
					}
				}
			}
			if c.Body() != nil && (c.Kind == ast.KindMethodDeclaration || c.Kind == ast.KindFunctionDeclaration || c.Kind == ast.KindGetAccessor || c.Kind == ast.KindSetAccessor) {
				if signature := ck.GetSignatureFromDeclaration(c); signature != nil {
					typeEdges(ck.GetReturnTypeOfSignature(signature))
				}
			}
			if c.Kind == ast.KindPropertyDeclaration || c.Kind == ast.KindPropertySignature || c.Kind == ast.KindParameter {
				typeEdges(ck.GetTypeAtLocation(c))
			}
			excluded := l.unexecuted[c] != ""
			c.ForEachChild(func(child *ast.Node) bool {
				if !values[n] && (c.Kind == ast.KindPropertyDeclaration || c.Kind == ast.KindVariableDeclaration || c.Kind == ast.KindParameter) && child == c.Initializer() {
					return false
				}
				if excluded && child == c.Body() {
					return false
				}
				if c.Kind == ast.KindImportDeclaration || c.Kind == ast.KindExportDeclaration {
					return false
				}
				walk(child)
				return false
			})
		}
		walk(n)
		done()
	}
	l.retained = retained
	l.typeOnly = map[*ast.Node]bool{}
	for n := range retained {
		if n.Kind != ast.KindClassDeclaration || values[n] {
			continue
		}
		l.typeOnly[n] = true
		f := owners[n]
		for _, m := range n.Members() {
			if m.Body() != nil {
				line, _ := lineCol(f, scanner.GetTokenPosOfNode(m, f, false))
				rel, _ := filepath.Rel(l.prog.configDir, f.FileName())
				l.unexecuted[m] = fmt.Sprintf("%s:%d", filepath.ToSlash(rel), line)
			}
		}
	}
	var selected []string
	count := 0
	for _, name := range files {
		f, _ := l.prog.File(name)
		live := false
		for _, n := range f.Statements.Nodes {
			if retained[n] {
				live = true
				count++
			}
		}
		if live {
			selected = append(selected, name)
		}
	}
	l.diags = append(l.diags, LowerDiagnostic{Category: "note-declaration-reachability", Message: fmt.Sprintf("retained %d top-level declarations in %d of %d source files", count, len(selected), len(files))})
	return selected, nil
}

func (l *lowerer) typeOnlyLocation(n *ast.Node) string {
	f := ast.GetSourceFileOfNode(n)
	rel, _ := filepath.Rel(l.prog.configDir, f.FileName())
	line, _ := lineCol(f, scanner.GetTokenPosOfNode(n, f, false))
	return fmt.Sprintf("%s:%d", filepath.ToSlash(rel), line)
}
