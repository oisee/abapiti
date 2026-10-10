package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func enclosingFunction(n *ast.Node) *ast.Node {
	for ; n != nil; n = n.Parent {
		if ast.IsFunctionLike(n) {
			return n
		}
	}
	return nil
}

func declarationStatement(n *ast.Node) *ast.Node {
	for ; n != nil && !ast.IsFunctionLike(n); n = n.Parent {
		if n.Kind == ast.KindVariableStatement {
			return n
		}
	}
	return nil
}

// Use checker symbol identity, not spelling: shadowed locals and declarations
// in other functions must not turn independent failures into cascades.
func (l *lowerer) classifyCascades() {
	failed := map[*ast.Node]bool{}
	for _, d := range l.diags {
		if !strings.HasPrefix(d.Category, "note-") {
			if s := declarationStatement(d.node); s != nil {
				failed[s] = true
			}
		}
	}
	for i := range l.diags {
		d := &l.diags[i]
		if d.node == nil || d.node.Kind != ast.KindIdentifier || !strings.HasPrefix(d.Message, "identifier ") || !strings.HasSuffix(d.Message, " is not a lowered local or module value") {
			continue
		}
		sym := l.ck.GetSymbolAtLocation(d.node)
		if sym == nil || sym.ValueDeclaration == nil {
			continue
		}
		s := declarationStatement(sym.ValueDeclaration)
		f := enclosingFunction(d.node)
		if s != nil && failed[s] && f != nil && f == enclosingFunction(s) && s.End() <= d.node.Pos() {
			d.Cascade = true
		}
	}
}
