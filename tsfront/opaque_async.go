package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Check declaration provenance: user classes named Promise/Map/Set must
// retain their own semantics.
func (l *lowerer) librarySymbol(sym *ast.Symbol) bool {
	if sym == nil {
		return false
	}
	if l.file != nil && l.classesByName[l.file.FileName()+" "+sym.Name] != nil {
		return false
	}
	if len(sym.Declarations) == 0 {
		return false
	}
	// Global user declarations can merge with a library symbol. Checking only
	// its first declaration would mistake a user constructor for the builtin.
	for _, declaration := range sym.Declarations {
		f := ast.GetSourceFileOfNode(declaration)
		if f == nil || !l.prog.prog.IsSourceFileDefaultLibrary(f.Path()) {
			return false
		}
	}
	return true
}

func (l *lowerer) opaquePromise() string {
	const name = "builtin.OpaquePromise"
	for _, c := range l.out.Classes {
		if c.Name == name {
			return name
		}
	}
	// Abstract with no API or constructor: a trapped body may mention this
	// result type but cannot produce a successfully callable Promise.
	l.out.Classes = append(l.out.Classes, &hir.Class{Name: name, Abstract: true})
	return name
}
