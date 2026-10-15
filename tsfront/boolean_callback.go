package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Boolean as a callback ignores the index and array arguments, and returns
// ordinary JavaScript truthiness of the element. Check identity, not spelling.
func (l *lowerer) booleanCallback(n, cb *ast.Node, elem hir.Type) (*hir.Expr, []string, bool) {
	if cb.Kind != ast.KindIdentifier {
		return nil, nil, false
	}
	sym := l.resolve(cb)
	if sym == nil || sym.Name != "Boolean" || !l.librarySymbol(sym) {
		return nil, nil, false
	}
	l.serial++
	name := "$boolean" + itoa(l.serial)
	return l.toBool(n, hir.V(name, elem)), []string{name}, true
}
