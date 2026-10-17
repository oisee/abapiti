package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// arrayInterfaceElement reports a user interface that extends the library
// Array<T> (or ReadonlyArray<T>), such as OSG's `interface
// FrontendSelectColumns extends Array<FrontendSelectColumn> { grouped?:
// boolean }`, and returns T. Its values are JavaScript arrays: it lowers to
// the array ABI, never to a data class that would adopt Array's own member
// symbols (length, push, ...) as fields.
func (l *lowerer) arrayInterfaceElement(t *checker.Type) (*checker.Type, bool) {
	if !l.unprunedFeatures || l.ck == nil || t == nil || t.Symbol() == nil || t.Symbol().Flags&ast.SymbolFlagsInterface == 0 || t.ObjectFlags()&checker.ObjectFlagsClassOrInterface == 0 {
		return nil, false
	}
	if l.librarySymbol(t.Symbol()) {
		return nil, false
	}
	for _, d := range t.Symbol().Declarations {
		if d.Kind != ast.KindInterfaceDeclaration {
			return nil, false
		}
	}
	for _, base := range l.ck.GetBaseTypes(t) {
		if base == nil {
			continue
		}
		if l.ck.IsArrayType(base) {
			if args := l.ck.GetTypeArguments(base); len(args) == 1 {
				return args[0], true
			}
		}
		if elem, ok := l.arrayInterfaceElement(base); ok {
			return elem, true
		}
	}
	return nil, false
}

// arrayInterfaceDeclaration is arrayInterfaceElement for an interface node.
func (l *lowerer) arrayInterfaceDeclaration(node *ast.Node) bool {
	if node == nil || node.Kind != ast.KindInterfaceDeclaration || node.AsInterfaceDeclaration().HeritageClauses == nil || node.Symbol() == nil || l.ck == nil {
		return false
	}
	_, ok := l.arrayInterfaceElement(l.ck.GetDeclaredTypeOfSymbol(node.Symbol()))
	return ok
}

func (l *lowerer) arrayInterfaceType(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	elem, ok := l.arrayInterfaceElement(t)
	if !ok {
		return hir.Type{}, false
	}
	return hir.T(hir.Array, l.mapCheckerType(n, elem)), true
}
