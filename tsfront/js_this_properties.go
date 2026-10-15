package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// jsThisProperties declares the instance properties a JavaScript class
// creates by assignment (this.x = …, optionally typed with JSDoc @type), as
// TypeScript's checker does for .js/.mjs files. Each becomes a field with the
// checker's property type, in first-assignment order; TS files and names
// already declared as members are left alone.
func (l *lowerer) jsThisProperties(node *ast.Node, c *hir.Class) {
	name := l.file.FileName()
	if !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".mjs") && !strings.HasSuffix(name, ".cjs") {
		return
	}
	known := map[string]bool{}
	for _, f := range c.Fields {
		known[f.Name] = true
	}
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n == nil {
			return false
		}
		switch n.Kind {
		case ast.KindClassDeclaration, ast.KindClassExpression, ast.KindFunctionDeclaration, ast.KindFunctionExpression:
			return false // another this
		case ast.KindBinaryExpression:
			b := n.AsBinaryExpression()
			if b.OperatorToken.Kind == ast.KindEqualsToken && b.Left.Kind == ast.KindPropertyAccessExpression {
				pa := b.Left.AsPropertyAccessExpression()
				if pa.Expression.Kind == ast.KindThisKeyword && pa.Name().Kind == ast.KindIdentifier {
					field := pa.Name().Text()
					if !known[field] {
						if t := l.ck.GetTypeAtLocation(b.Left); t != nil {
							typ := l.mapCheckerType(b.Left, t)
							if typ.Kind != hir.Void {
								known[field] = true
								c.Fields = append(c.Fields, hir.Field{Node: l.node(b.Left), Name: field, Type: typ})
							}
						}
					}
				}
			}
		}
		n.ForEachChild(visit)
		return false
	}
	for _, m := range node.Members() {
		if (m.Kind == ast.KindConstructor || m.Kind == ast.KindMethodDeclaration) && m.Body() != nil && m.ModifierFlags()&ast.ModifierFlagsStatic == 0 {
			m.Body().ForEachChild(visit)
		}
	}
}
