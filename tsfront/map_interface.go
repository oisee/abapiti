package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Interfaces that refine a library Map's signatures retain its collection
// storage. In particular, an overloaded get does not turn Map into a record
// of function-valued fields or adopt the library's member symbols.
func (l *lowerer) mapInterfaceBase(t *checker.Type) *checker.Type {
	if !l.unprunedFeatures || l.ck == nil || t == nil || t.Symbol() == nil || t.Symbol().Flags&ast.SymbolFlagsInterface == 0 || t.ObjectFlags()&checker.ObjectFlagsClassOrInterface == 0 || l.librarySymbol(t.Symbol()) {
		return nil
	}
	for _, d := range t.Symbol().Declarations {
		if d.Kind != ast.KindInterfaceDeclaration {
			return nil
		}
	}
	for _, base := range l.ck.GetBaseTypes(t) {
		if base == nil {
			continue
		}
		if sym := base.Symbol(); sym != nil && l.librarySymbol(sym) && (sym.Name == "Map" || sym.Name == "ReadonlyMap") {
			return base
		}
		if found := l.mapInterfaceBase(base); found != nil {
			return found
		}
	}
	return nil
}

func (l *lowerer) mapInterfaceDeclaration(n *ast.Node) bool {
	return n != nil && n.Kind == ast.KindInterfaceDeclaration && n.Symbol() != nil && n.AsInterfaceDeclaration().HeritageClauses != nil && l.mapInterfaceBase(l.ck.GetDeclaredTypeOfSymbol(n.Symbol())) != nil
}

func (l *lowerer) mapInterfaceType(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if base := l.mapInterfaceBase(t); base != nil {
		return l.mapCheckerType(n, base), true
	}
	return hir.Type{}, false
}
