package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/core"
	"github.com/oisee/abapiti/internal/tsgo/parser"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// CertificateSpans uses the override key and token-span convention exactly.
// It is a read-only source index, independent of lowering and both emitters.
func CertificateSpans(file, source string) (map[overrides.Key]string, error) {
	f := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/" + file}, source, core.ScriptKindTS)
	if len(f.Diagnostics()) > 0 {
		return nil, fmt.Errorf("certificate source does not parse: %s", file)
	}
	out := map[overrides.Key]string{}
	var walk func(*ast.Node, string)
	walk = func(n *ast.Node, symbol string) {
		switch n.Kind {
		case ast.KindClassDeclaration, ast.KindInterfaceDeclaration, ast.KindFunctionDeclaration, ast.KindMethodDeclaration, ast.KindMethodSignature, ast.KindPropertyDeclaration, ast.KindVariableDeclaration, ast.KindConstructor:
			if n.Kind == ast.KindConstructor {
				symbol += ".constructor"
			} else if n.Name() != nil && (n.Name().Kind == ast.KindIdentifier || n.Name().Kind == ast.KindStringLiteral) {
				if (n.Kind == ast.KindMethodDeclaration || n.Kind == ast.KindMethodSignature || n.Kind == ast.KindPropertyDeclaration) && symbol != "" {
					symbol += "." + n.Name().Text()
				} else {
					symbol = n.Name().Text()
				}
			}
			key := overrides.Key{File: file, Symbol: symbol, Kind: n.Kind.String()}
			span := source[scanner.GetTokenPosOfNode(n, f, false):n.End()]
			if _, ok := out[key]; ok {
				out[key] = ""
			} else {
				out[key] = span
			}
		}
		n.ForEachChild(func(c *ast.Node) bool { walk(c, symbol); return false })
	}
	for _, n := range f.Statements.Nodes {
		walk(n, "")
	}
	return out, nil
}
